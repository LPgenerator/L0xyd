package commands

import (
	"os"
	"io"
	"fmt"
	"time"
	"strconv"
	"strings"
	"net/url"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"encoding/base64"

	"github.com/thoas/stats"
	"github.com/codegangsta/cli"

	"github.com/mailgun/oxy/utils"
	"github.com/mailgun/oxy/trace"
	"github.com/mailgun/oxy/stream"
	"github.com/mailgun/oxy/forward"
	"github.com/codegangsta/negroni"
	"github.com/mailgun/oxy/connlimit"
	"github.com/mailgun/oxy/ratelimit"
	"github.com/mailgun/oxy/roundrobin"

	log "github.com/Sirupsen/logrus"
	"github.com/LPgenerator/L0xyd/common"
	"github.com/LPgenerator/L0xyd/helpers"
	service "github.com/ayufan/golang-kardianos-service"
	"github.com/LPgenerator/L0xyd/helpers/service"
	"github.com/LPgenerator/L0xyd/commands/mirroring"
	"github.com/LPgenerator/L0xyd/commands/statistics"
	"github.com/LPgenerator/L0xyd/commands/monitoring"
	"github.com/LPgenerator/L0xyd/commands/x-headers"
	"github.com/LPgenerator/L0xyd/commands/websockets"

)


type Server struct {
	Url              string
	Weight           int
}

type RunCommand struct {
	configOptions

	ListenAddr       string `short:"l" long:"listen" description:"Listen address:port"`
	ServiceName      string `short:"n" long:"service" description:"Use different names for different services"`
	WorkingDirectory string `short:"d" long:"working-directory" description:"Specify custom working directory"`
	Syslog           bool   `long:"syslog" description:"Log to syslog"`
}

var LB struct {
	lb               *roundrobin.RoundRobin
	rl               http.Handler
	config           *common.Config
	stats            *stats.Stats
	mirror           *mirror.Mirroring
	stream           *stream.Streamer
}


func setStatus(w http.ResponseWriter, status string) {
	if status == "ERROR" {
		http.Error(
			w, fmt.Sprintf(`{"status": "%s"}`, status),
			http.StatusInternalServerError)
	} else {
		// w.WriteHeader(http.StatusCreated)
		io.WriteString(w, fmt.Sprintf(`{"status": "%s"}`, status))
	}
}

func removeServerFromConfig(s string) {
	for serverName, server := range LB.config.Servers {
		if server.Url == fmt.Sprintf("http://%s", s) {
			delete(LB.config.Servers, serverName)
			LB.mirror.Del("http://" + s)
		}
	}
}

func addServerToConfig(s string, v int, t string) {
	counter := 1
	for {
		val := LB.config.Servers[fmt.Sprintf("web-%d", counter)]
		if val.Url == "" {
			break
		} else {
			counter = counter + 1
		}
	}
	web := fmt.Sprintf("web-%d", counter)
	LB.config.Servers[web] = common.Server{
		Url: "http://" + s,
		Weight: v,
		Type: t,
	}
	if t == "mirror" {
		LB.mirror.Add("http://" + s)
	}
}

func setHttpHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Server", "L0xyd")
}

func getNextHandler(new http.Handler, old http.Handler, enabled bool, mw string) (handler http.Handler) {
	if enabled == true {
		log.Printf("LB: '%s' is enabled", mw)
		return new
	}
	return old
}

func removeBackendHandler(w http.ResponseWriter, backend string) {
	u, err := url.Parse("http://" + backend)
	if err != nil {
		setStatus(w, "ERROR")
	} else {
		srv_type := "standard"
		for _, server := range LB.config.Servers {
			if server.Url == fmt.Sprintf("http://%s", backend) {
				srv_type = server.Type
			}
		}
		err := LB.lb.RemoveServer(u)
		if (srv_type == "standard" || srv_type == "") && err != nil {
			setStatus(w, "ERROR")
			log.Errorf("failed to remove %s, err: %v", backend, err)
		} else {
			removeServerFromConfig(backend)
			setStatus(w, "OK")
			log.Infof("%s was removed", backend)
		}
	}
}

func HandleApiIndex(w http.ResponseWriter, r *http.Request) {
	setHttpHeaders(w)
	if r.Method == "PUT" || r.Method == "POST" {
		HandleAdd(w, r)
	} else if r.Method == "DELETE" {
		HandleDel(w, r)
	} else {
		HandleList(w, r)
	}
}

func HandleAdd(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	s := r.Form.Get("url")
	t := r.Form.Get("type")
	v, err := strconv.Atoi(r.Form.Get("weight"))
	if err != nil { v = 0 }
	if s != "" {
		u, err := url.Parse("http://" + s)
		if err != nil {
			setStatus(w, "ERROR")
		} else {
			var err error = nil
			if t == "" || t == "standard" {
				err = LB.lb.UpsertServer(u, roundrobin.Weight(v));
			}
			if  err != nil {
				setStatus(w, "ERROR")
				log.Errorf("failed to add %s, err: %s", s, err)
			} else {
				removeServerFromConfig(s)
				addServerToConfig(s, v, t)
				setStatus(w, "OK")
				log.Infof("%s was added. weight: %d", s, v)
			}
		}
	} else {
		setStatus(w, "ERROR")
	}
}

func HandleDel(w http.ResponseWriter, r *http.Request) {
	s := strings.Split(r.URL.Path, "/")
	if len(s) == 2 {
		removeBackendHandler(w, s[1])
	} else {
		setStatus(w, "ERROR")
	}
}

func HandleList(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(LB.config.Servers)
	if err == nil {
		io.WriteString(w, string(data))
	} else {
		setStatus(w, "ERROR")
	}
}

func HandleApiStats(w http.ResponseWriter, r *http.Request) {
	data, err := json.MarshalIndent(LB.stats.Data(), "", "  ")
	setHttpHeaders(w)
	if err == nil {
		io.WriteString(w, string(data))
	} else {
		setStatus(w, "ERROR")
	}
}

func HandleApiStatus(w http.ResponseWriter, r *http.Request) {
	updateSystemStatus()
	setHttpHeaders(w)

	stats, err := json.MarshalIndent(sysStatus, "", "  ")
	if err == nil {
		io.WriteString(w, string(stats))
	} else {
		setStatus(w, "ERROR")
	}
}

const htmlData = "PCFkb2N0eXBlIGh0bWw+Cgo8aHRtbD4KCjxoZWFkPgogICAgPHRpdGxlPkwweHlkPC90aXRsZT4KICAgIDxtZXRhIG5hbWU9InZpZXdwb3J0IiBjb250ZW50PSJ3aWR0aD1kZXZpY2Utd2lkdGgiPgogICAgPGxpbmsgcmVsPSJzdHlsZXNoZWV0IiBocmVmPSJodHRwczovL25ldGRuYS5ib290c3RyYXBjZG4uY29tL2Jvb3Rzd2F0Y2gvMy4wLjAvam91cm5hbC9ib290c3RyYXAubWluLmNzcyI+CiAgICA8bGluayByZWw9InN0eWxlc2hlZXQiIHR5cGU9InRleHQvY3NzIiBtZWRpYT0ic2NyZWVuIgogICAgICAgICAgaHJlZj0iaHR0cDovL3d3dy5ndXJpZGRvLm5ldC9kZW1vL2Nzcy90cmlyYW5kL3VpLmpxZ3JpZC1ib290c3RyYXAuY3NzIj4KICAgIDxzY3JpcHQgdHlwZT0idGV4dC9qYXZhc2NyaXB0IiBzcmM9Imh0dHBzOi8vYWpheC5nb29nbGVhcGlzLmNvbS9hamF4L2xpYnMvanF1ZXJ5LzIuMC4zL2pxdWVyeS5taW4uanMiPjwvc2NyaXB0PgogICAgPHNjcmlwdCB0eXBlPSJ0ZXh0L2phdmFzY3JpcHQiIHNyYz0iaHR0cHM6Ly9uZXRkbmEuYm9vdHN0cmFwY2RuLmNvbS9ib290c3RyYXAvMy4zLjQvanMvYm9vdHN0cmFwLm1pbi5qcyI+PC9zY3JpcHQ+CiAgICA8c2NyaXB0IHR5cGU9InRleHQvamF2YXNjcmlwdCIgc3JjPSJodHRwOi8vd3d3Lmd1cmlkZG8ubmV0L2RlbW8vanMvdHJpcmFuZC9qcXVlcnkuanFHcmlkLm1pbi5qcyI+PC9zY3JpcHQ+CiAgICA8c2NyaXB0IHR5cGU9InRleHQvamF2YXNjcmlwdCIgc3JjPSJodHRwOi8vd3d3Lmd1cmlkZG8ubmV0L2RlbW8vanMvdHJpcmFuZC9pMThuL2dyaWQubG9jYWxlLWVuLmpzIj48L3NjcmlwdD4KICAgIDxsaW5rIHJlbD0ic3R5bGVzaGVldCIgaHJlZj0iLy9jb2RlLmpxdWVyeS5jb20vdWkvMS4xMS40L3RoZW1lcy9zbW9vdGhuZXNzL2pxdWVyeS11aS5jc3MiPgogICAgPHNjcmlwdCBzcmM9Imh0dHA6Ly9jb2RlLmpxdWVyeS5jb20vdWkvMS4xMS40L2pxdWVyeS11aS5qcyI+PC9zY3JpcHQ+CiAgICA8c3R5bGUgdHlwZT0idGV4dC9jc3MiPgogICAgICAgIGJvZHkgewogICAgICAgICAgICBwYWRkaW5nLXRvcDogNTBweDsKICAgICAgICAgICAgcGFkZGluZy1ib3R0b206IDIwcHg7CiAgICAgICAgfQogICAgPC9zdHlsZT4KICAgIDxzY3JpcHQgdHlwZT0idGV4dC9qYXZhc2NyaXB0Ij4KICAgICAgICBmdW5jdGlvbiBVcGRhdGVUYWJsZSgpIHsKICAgICAgICAgICAgJCgiI2pxR3JpZCIpCiAgICAgICAgICAgICAgICAuanFHcmlkKHsKICAgICAgICAgICAgICAgICAgICB1cmw6ICcvZGF0YS5qc29uJywKICAgICAgICAgICAgICAgICAgICBtdHlwZTogIkdFVCIsCiAgICAgICAgICAgICAgICAgICAgYWpheFN1YmdyaWRPcHRpb25zOiB7CiAgICAgICAgICAgICAgICAgICAgICAgIGFzeW5jOiBmYWxzZQogICAgICAgICAgICAgICAgICAgIH0sCiAgICAgICAgICAgICAgICAgICAgc3R5bGVVSTogJ0Jvb3RzdHJhcCcsCiAgICAgICAgICAgICAgICAgICAgZGF0YXR5cGU6ICJqc29uIiwKICAgICAgICAgICAgICAgICAgICBjb2xNb2RlbDogW3sKICAgICAgICAgICAgICAgICAgICAgICAgbGFiZWw6ICcjJywKICAgICAgICAgICAgICAgICAgICAgICAgbmFtZTogJ2lkJywKICAgICAgICAgICAgICAgICAgICAgICAgd2lkdGg6IDUKICAgICAgICAgICAgICAgICAgICB9LCB7CiAgICAgICAgICAgICAgICAgICAgICAgIGxhYmVsOiAnQmFja2VuZCcsCiAgICAgICAgICAgICAgICAgICAgICAgIG5hbWU6ICdiYWNrZW5kJywKICAgICAgICAgICAgICAgICAgICAgICAga2V5OiB0cnVlLAogICAgICAgICAgICAgICAgICAgICAgICB3aWR0aDogMTUKICAgICAgICAgICAgICAgICAgICB9LCB7CiAgICAgICAgICAgICAgICAgICAgICAgIGxhYmVsOiAnV2VpZ2h0JywKICAgICAgICAgICAgICAgICAgICAgICAgbmFtZTogJ3dlaWdodCcsCiAgICAgICAgICAgICAgICAgICAgICAgIHdpZHRoOiAyMAogICAgICAgICAgICAgICAgICAgIH0sIHsKICAgICAgICAgICAgICAgICAgICAgICAgbGFiZWw6ICdUeXBlJywKICAgICAgICAgICAgICAgICAgICAgICAgbmFtZTogJ3R5cGUnLAogICAgICAgICAgICAgICAgICAgICAgICB3aWR0aDogMjAKICAgICAgICAgICAgICAgICAgICB9XSwKICAgICAgICAgICAgICAgICAgICB2aWV3cmVjb3JkczogdHJ1ZSwKICAgICAgICAgICAgICAgICAgICByb3dOdW06IDIwLAogICAgICAgICAgICAgICAgICAgIHBhZ2VyOiAiI2pxR3JpZFBhZ2VyIgogICAgICAgICAgICAgICAgfSkKICAgICAgICB9CgogICAgICAgIGZ1bmN0aW9uIEZpeFRhYmxlKCkgewogICAgICAgICAgICAkLmV4dGVuZCgkLmpncmlkLmFqYXhPcHRpb25zLCB7CiAgICAgICAgICAgICAgICBhc3luYzogZmFsc2UKICAgICAgICAgICAgfSk7CiAgICAgICAgICAgIHZhciBncmlkID0gJCgiI2pxR3JpZCIpOwogICAgICAgICAgICBncmlkLnNldEdyaWRXaWR0aCgkKHdpbmRvdykud2lkdGgoKSAtIDUpOwogICAgICAgICAgICBncmlkLnNldEdyaWRIZWlnaHQoJCh3aW5kb3cpLmhlaWdodCgpKTsKICAgICAgICAgICAgJCh3aW5kb3cpLmJpbmQoJ3Jlc2l6ZScsIGZ1bmN0aW9uKCkgewogICAgICAgICAgICAgICAgdmFyIGpxX2dyaWQgPSAkKCIjanFHcmlkIik7CiAgICAgICAgICAgICAgICBqcV9ncmlkLnNldEdyaWRXaWR0aCgkKHdpbmRvdykud2lkdGgoKSAtIDUpOwogICAgICAgICAgICAgICAganFfZ3JpZC5zZXRHcmlkSGVpZ2h0KCQod2luZG93KS5oZWlnaHQoKSk7CiAgICAgICAgICAgIH0pOwogICAgICAgIH0KCiAgICAgICAgZnVuY3Rpb24gVmFsaWRhdGVCYWNrZW5kKGJhY2tlbmQpIHsKICAgICAgICAgICAgcmV0dXJuIGJhY2tlbmQubWF0Y2goL14oPzpbMC05XXsxLDN9XC4pezN9WzAtOV17MSwzfTpbMC05XXsyLDV9JC8pOwogICAgICAgIH0KCiAgICAgICAgZnVuY3Rpb24gQWRkQmFja2VuZCgpIHsKICAgICAgICAgICAgdmFyIHVybCA9ICQoIiN1cmxfaWQiKS52YWwoKS5yZXBsYWNlKCJodHRwOi8vIiwgIiIpOwogICAgICAgICAgICB2YXIgd2VpZ2h0ID0gJCgiI3dlaWdodF9pZCIpLnZhbCgpOwoKICAgICAgICAgICAgaWYgKCFWYWxpZGF0ZUJhY2tlbmQodXJsKSkgewogICAgICAgICAgICAgICAgcmV0dXJuIHNob3dFcnJvcigiSW52YWxpZCBiYWNrZW5kISIpCiAgICAgICAgICAgIH0KCiAgICAgICAgICAgIGlmICghd2VpZ2h0Lm1hdGNoKC9eWzAtOV17MSwyfSQvKSkgewogICAgICAgICAgICAgICAgcmV0dXJuIHNob3dFcnJvcigiSW52YWxpZCB3ZWlnaHQhIikKICAgICAgICAgICAgfQoKICAgICAgICAgICAgdmFyIHJlcSA9IHsKICAgICAgICAgICAgICAgIHdlaWdodDogcGFyc2VJbnQod2VpZ2h0KSwKICAgICAgICAgICAgICAgIHVybDogdXJsLAogICAgICAgICAgICAgICAgdHlwZTogJCgiI3R5cGVfaWQiKS52YWwoKQogICAgICAgICAgICB9OwoKICAgICAgICAgICAgJC5hamF4KHsKICAgICAgICAgICAgICAgIHVybDogIi9hZGRfYmFja2VuZCIsCiAgICAgICAgICAgICAgICB0eXBlOiAiUE9TVCIsCiAgICAgICAgICAgICAgICBkYXRhOiByZXEsCiAgICAgICAgICAgICAgICBkYXRhVHlwZTogImpzb24iCiAgICAgICAgICAgIH0pLmRvbmUoZnVuY3Rpb24oZGF0YSkgewogICAgICAgICAgICAgICAgaWYgKHR5cGVvZiAoZGF0YS5zdGF0dXMpID09ICJ1bmRlZmluZWQiIHx8IGRhdGEuc3RhdHVzICE9ICJPSyIpIHsKICAgICAgICAgICAgICAgICAgICBzaG93RXJyb3IoIkVycm9yIG9jY3VycmVkIC4uLiIpOwogICAgICAgICAgICAgICAgfSBlbHNlIHsKICAgICAgICAgICAgICAgICAgICBVcGRhdGVEYXRhKCk7CiAgICAgICAgICAgICAgICB9CiAgICAgICAgICAgIH0pLmVycm9yKGZ1bmN0aW9uKCkgewogICAgICAgICAgICAgICAgc2hvd0Vycm9yKCJDb25uZWN0aW9uIGVycm9yIC4uLiIpOwogICAgICAgICAgICB9KTsKICAgICAgICB9CgogICAgICAgIGZ1bmN0aW9uIFJlbW92ZUJhY2tlbmQoKSB7CiAgICAgICAgICAgIHZhciBncmlkID0gJCgiI2pxR3JpZCIpOwogICAgICAgICAgICB2YXIgcm93S2V5ID0gZ3JpZC5qcUdyaWQoJ2dldEdyaWRQYXJhbScsICJzZWxyb3ciKTsKCiAgICAgICAgICAgIGlmIChyb3dLZXkgPT0gbnVsbCkgewogICAgICAgICAgICAgICAgcmV0dXJuIHNob3dFcnJvcigiQmFja2VuZCBpcyBub3Qgc2VsZWN0ZWQhIikKICAgICAgICAgICAgfQoKICAgICAgICAgICAgdmFyIHVybCA9IHJvd0tleS5yZXBsYWNlKCJodHRwOi8vIiwgIiIpOwogICAgICAgICAgICBpZiAoIVZhbGlkYXRlQmFja2VuZCh1cmwpKSB7CiAgICAgICAgICAgICAgICByZXR1cm4gc2hvd0Vycm9yKCJJbnZhbGlkIGJhY2tlbmQhIikKICAgICAgICAgICAgfQoKICAgICAgICAgICAgJC5hamF4KHsKICAgICAgICAgICAgICAgIHVybDogIi9yZW1vdmVfYmFja2VuZCIsCiAgICAgICAgICAgICAgICB0eXBlOiAiUE9TVCIsCiAgICAgICAgICAgICAgICBkYXRhOiB7YmFja2VuZDogdXJsfSwKICAgICAgICAgICAgICAgIGRhdGFUeXBlOiAianNvbiIKICAgICAgICAgICAgfSkuZG9uZShmdW5jdGlvbihkYXRhKSB7CiAgICAgICAgICAgICAgICBpZiAodHlwZW9mIChkYXRhLnN0YXR1cykgPT0gInVuZGVmaW5lZCIgfHwgZGF0YS5zdGF0dXMgIT0gIk9LIikgewogICAgICAgICAgICAgICAgICAgIHNob3dFcnJvcigiRXJyb3Igb2NjdXJyZWQgLi4uIik7CiAgICAgICAgICAgICAgICB9IGVsc2UgewogICAgICAgICAgICAgICAgICAgIFVwZGF0ZURhdGEoKTsKICAgICAgICAgICAgICAgIH0KICAgICAgICAgICAgfSkuZXJyb3IoZnVuY3Rpb24oKSB7CiAgICAgICAgICAgICAgICBzaG93RXJyb3IoIkNvbm5lY3Rpb24gZXJyb3IgLi4uIik7CiAgICAgICAgICAgIH0pOwogICAgICAgIH0KCiAgICAgICAgZnVuY3Rpb24gVXBkYXRlRGF0YSgpIHsKICAgICAgICAgICAgdmFyIGdyaWQgPSAkKCIjanFHcmlkIik7CiAgICAgICAgICAgIHZhciByb3dLZXkgPSBncmlkLmpxR3JpZCgnZ2V0R3JpZFBhcmFtJywgInNlbHJvdyIpOwogICAgICAgICAgICBncmlkLnRyaWdnZXIoInJlbG9hZEdyaWQiKTsKICAgICAgICAgICAgaWYocm93S2V5KSB7CiAgICAgICAgICAgICAgICBncmlkLmpxR3JpZCgicmVzZXRTZWxlY3Rpb24iKTsKICAgICAgICAgICAgICAgIGdyaWQuanFHcmlkKCdzZXRTZWxlY3Rpb24nLCByb3dLZXkpOwogICAgICAgICAgICB9CiAgICAgICAgfQoKICAgICAgICBmdW5jdGlvbiBVcGRhdGVTdGF0cygpIHsKICAgICAgICAgICAgJC5hamF4KHsKICAgICAgICAgICAgICAgIHVybDogIi9zdGF0cy5qc29uIiwKICAgICAgICAgICAgICAgIHR5cGU6ICJHRVQiLAogICAgICAgICAgICAgICAgZGF0YVR5cGU6ICJ0ZXh0IgogICAgICAgICAgICB9KS5kb25lKGZ1bmN0aW9uKGRhdGEpIHsKICAgICAgICAgICAgICAgICQoJyNzdGF0cycpLnRleHQoZGF0YSk7CiAgICAgICAgICAgIH0pOwoKICAgICAgICB9CgogICAgICAgIGZ1bmN0aW9uIHNob3dFcnJvcihtZXNzYWdlKSB7CiAgICAgICAgICAgICQoIiNlcnJvck1lc3NhZ2UiKS5tb2RhbCgic2hvdyIpOwogICAgICAgICAgICAkKCIjZXJyb3ItbWVzc2FnZSIpLmh0bWwobWVzc2FnZSk7CiAgICAgICAgfQoKICAgICAgICBmdW5jdGlvbiBPbkxvYWQoKSB7CiAgICAgICAgICAgIFVwZGF0ZVRhYmxlKCk7CiAgICAgICAgICAgIEZpeFRhYmxlKCk7CiAgICAgICAgICAgIFVwZGF0ZVN0YXRzKCk7CiAgICAgICAgICAgIHNldEludGVydmFsKFVwZGF0ZURhdGEsIDE1MDAwKTsKICAgICAgICAgICAgc2V0SW50ZXJ2YWwoVXBkYXRlU3RhdHMsIDUwMDApOwogICAgICAgIH0KICAgIDwvc2NyaXB0Pgo8L2hlYWQ+Cgo8Ym9keSBvbmxvYWQ9Ik9uTG9hZCgpIj4KPGRpdiBjbGFzcz0ibmF2YmFyIG5hdmJhci1pbnZlcnNlIG5hdmJhci1maXhlZC10b3AiPgogICAgPGRpdiBjbGFzcz0iY29udGFpbmVyIj4KICAgICAgICA8ZGl2IGNsYXNzPSJuYXZiYXItaGVhZGVyIj4KICAgICAgICAgICAgPGJ1dHRvbiB0eXBlPSJidXR0b24iIGNsYXNzPSJuYXZiYXItdG9nZ2xlIiBkYXRhLXRvZ2dsZT0iY29sbGFwc2UiIGRhdGEtdGFyZ2V0PSIubmF2YmFyLWNvbGxhcHNlIj4KICAgICAgICAgICAgICAgIDxzcGFuIGNsYXNzPSJpY29uLWJhciI+PC9zcGFuPjxzcGFuIGNsYXNzPSJpY29uLWJhciI+PC9zcGFuPjxzcGFuIGNsYXNzPSJpY29uLWJhciI+PC9zcGFuPgogICAgICAgICAgICA8L2J1dHRvbj4KICAgICAgICAgICAgPGEgY2xhc3M9Im5hdmJhci1icmFuZCIgaHJlZj0iIyI+TDB4eWQgV0VCPC9hPgogICAgICAgIDwvZGl2PgogICAgICAgIDxkaXYgY2xhc3M9Im5hdmJhci1jb2xsYXBzZSBjb2xsYXBzZSI+CiAgICAgICAgICAgIDx1bCBjbGFzcz0ibmF2IG5hdmJhci1uYXYiPgogICAgICAgICAgICAgICAgPGxpIGNsYXNzPSJkcm9wZG93biI+CiAgICAgICAgICAgICAgICAgICAgPGEgaHJlZj0iIyIgY2xhc3M9ImRyb3Bkb3duLXRvZ2dsZSIgZGF0YS10b2dnbGU9ImRyb3Bkb3duIj5CYWNrZW5kcyA8YiBjbGFzcz0iY2FyZXQiPjwvYj48L2E+CiAgICAgICAgICAgICAgICAgICAgPHVsIGNsYXNzPSJkcm9wZG93bi1tZW51Ij4KICAgICAgICAgICAgICAgICAgICAgICAgPGxpPgogICAgICAgICAgICAgICAgICAgICAgICAgICAgPGEgZGF0YS10b2dnbGU9Im1vZGFsIiBkYXRhLXRhcmdldD0iI2FkZEJhY2tlbmQiPkFkZCBCYWNrZW5kPC9hPgogICAgICAgICAgICAgICAgICAgICAgICA8L2xpPgogICAgICAgICAgICAgICAgICAgICAgICA8bGkgb25jbGljaz0iUmVtb3ZlQmFja2VuZCgpIj4KICAgICAgICAgICAgICAgICAgICAgICAgICAgIDxhIGhyZWY9IiMiPkRlbGV0ZSBCYWNrZW5kPC9hPgogICAgICAgICAgICAgICAgICAgICAgICA8L2xpPgogICAgICAgICAgICAgICAgICAgIDwvdWw+CiAgICAgICAgICAgICAgICA8L2xpPgogICAgICAgICAgICAgICAgPGxpPgogICAgICAgICAgICAgICAgICAgIDxhIGRhdGEtdG9nZ2xlPSJtb2RhbCIgZGF0YS10YXJnZXQ9IiNzaG93U3RhdGlzdGljcyI+U3RhdGlzdGljczwvYT4KICAgICAgICAgICAgICAgIDwvbGk+CiAgICAgICAgICAgICAgICA8bGk+CiAgICAgICAgICAgICAgICAgICAgPGEgZGF0YS10b2dnbGU9Im1vZGFsIiBkYXRhLXRhcmdldD0iI2Fib3V0V2luZG93Ij5BYm91dDwvYT4KICAgICAgICAgICAgICAgIDwvbGk+CiAgICAgICAgICAgIDwvdWw+CiAgICAgICAgPC9kaXY+CiAgICAgICAgPCEtLS8ubmF2YmFyLWNvbGxhcHNlIC0tPgogICAgPC9kaXY+CjwvZGl2Pgo8L1A+Cjx0YWJsZSBpZD0ianFHcmlkIj48L3RhYmxlPgoKPCEtLSBNb2RhbCAtLT4KPGRpdiBjbGFzcz0ibW9kYWwgZmFkZSIgaWQ9ImFkZEJhY2tlbmQiIHJvbGU9ImRpYWxvZyI+CiAgICA8ZGl2IGNsYXNzPSJtb2RhbC1kaWFsb2ciPgogICAgICAgIDwhLS0gTW9kYWwgY29udGVudC0tPgogICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWNvbnRlbnQiPgogICAgICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1oZWFkZXIiPgogICAgICAgICAgICAgICAgPGJ1dHRvbiB0eXBlPSJidXR0b24iIGNsYXNzPSJjbG9zZSIgZGF0YS1kaXNtaXNzPSJtb2RhbCI+JnRpbWVzOzwvYnV0dG9uPgogICAgICAgICAgICAgICAgPGg0IGNsYXNzPSJtb2RhbC10aXRsZSI+QmFja2VuZDwvaDQ+CiAgICAgICAgICAgIDwvZGl2PgogICAgICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1ib2R5Ij4KICAgICAgICAgICAgICAgIDxkaXYgY2xhc3M9ImZvcm0tZ3JvdXAiPgoKICAgICAgICAgICAgICAgICAgICA8bGFiZWwgY2xhc3M9ImNvbnRyb2wtbGFiZWwiPlVybDwvbGFiZWw+CiAgICAgICAgICAgICAgICAgICAgPGRpdiBjbGFzcz0iY29udHJvbHMiPgogICAgICAgICAgICAgICAgICAgICAgICA8aW5wdXQgdHlwZT0idGV4dCIgaWQ9InVybF9pZCIgY2xhc3M9ImZvcm0tY29udHJvbCIgdmFsdWU9Imh0dHA6Ly8xMjcuMC4wLjE6ODA4MSI+CiAgICAgICAgICAgICAgICAgICAgPC9kaXY+CgogICAgICAgICAgICAgICAgICAgIDxsYWJlbCBjbGFzcz0iY29udHJvbC1sYWJlbCI+V2VpZ2h0PC9sYWJlbD4KICAgICAgICAgICAgICAgICAgICA8ZGl2IGNsYXNzPSJjb250cm9scyI+CiAgICAgICAgICAgICAgICAgICAgICAgIDxpbnB1dCB0eXBlPSJ0ZXh0IiBpZD0id2VpZ2h0X2lkIiBjbGFzcz0iZm9ybS1jb250cm9sIiB2YWx1ZT0iMSI+CiAgICAgICAgICAgICAgICAgICAgPC9kaXY+CgogICAgICAgICAgICAgICAgICAgIDxsYWJlbCBjbGFzcz0iY29udHJvbC1sYWJlbCI+VHlwZTwvbGFiZWw+CiAgICAgICAgICAgICAgICAgICAgPGRpdiBjbGFzcz0iY29udHJvbHMiPgogICAgICAgICAgICAgICAgICAgICAgICA8c2VsZWN0IGNsYXNzPSJmb3JtLWNvbnRyb2wiIGlkPSJ0eXBlX2lkIj4KICAgICAgICAgICAgICAgICAgICAgICAgICAgIDxvcHRpb24+c3RhbmRhcmQ8L29wdGlvbj4KICAgICAgICAgICAgICAgICAgICAgICAgICAgIDxvcHRpb24+bWlycm9yPC9vcHRpb24+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICA8b3B0aW9uPmRvd248L29wdGlvbj4KICAgICAgICAgICAgICAgICAgICAgICAgICAgIDxvcHRpb24+YmFja3VwPC9vcHRpb24+CiAgICAgICAgICAgICAgICAgICAgICAgIDwvc2VsZWN0PgogICAgICAgICAgICAgICAgICAgIDwvZGl2PgoKICAgICAgICAgICAgICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1mb290ZXIiPgogICAgICAgICAgICAgICAgICAgICAgICA8YSBjbGFzcz0iYnRuIGJ0bi1wcmltYXJ5IiBvbmNsaWNrPSJBZGRCYWNrZW5kKCkiIGRhdGEtZGlzbWlzcz0ibW9kYWwiPkFkZCBiYWNrZW5kPC9hPgogICAgICAgICAgICAgICAgICAgIDwvZGl2PgogICAgICAgICAgICAgICAgPC9kaXY+CiAgICAgICAgICAgIDwvZGl2PgogICAgICAgIDwvZGl2PgogICAgPC9kaXY+CjwvZGl2PgoKPCEtLSBNb2RhbCAtLT4KPGRpdiBjbGFzcz0ibW9kYWwgZmFkZSIgaWQ9ImFib3V0V2luZG93IiByb2xlPSJkaWFsb2ciPgogICAgPGRpdiBjbGFzcz0ibW9kYWwtZGlhbG9nIj4KICAgICAgICA8IS0tIE1vZGFsIGNvbnRlbnQtLT4KICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1jb250ZW50Ij4KICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtaGVhZGVyIj4KICAgICAgICAgICAgICAgIDxidXR0b24gdHlwZT0iYnV0dG9uIiBjbGFzcz0iY2xvc2UiIGRhdGEtZGlzbWlzcz0ibW9kYWwiPiZ0aW1lczs8L2J1dHRvbj4KICAgICAgICAgICAgICAgIDxoNCBjbGFzcz0ibW9kYWwtdGl0bGUiPkFib3V0PC9oND4KICAgICAgICAgICAgPC9kaXY+CiAgICAgICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWJvZHkiPgogICAgICAgICAgICAgICAgPHA+CiAgICAgICAgICAgICAgICAgICAgPHN0cm9uZz5OQU1FOjwvc3Ryb25nPjwvYnI+CiAgICAgICAgICAgICAgICAgICAgJm5ic3A7Jm5ic3A7Jm5ic3A7Jm5ic3A7TDB4eWQgLSBTaW1wbGUgbG9hZCBiYWxhbmNlciB3aXRoIEh0dHAgQVBJLgogICAgICAgICAgICAgICAgPC9wPgogICAgICAgICAgICAgICAgPHA+CiAgICAgICAgICAgICAgICAgICAgPHN0cm9uZz5BVVRIT1IoUyk6PC9zdHJvbmc+PC9icj4KICAgICAgICAgICAgICAgICAgICAmbmJzcDsmbmJzcDsmbmJzcDsmbmJzcDtHb1RMaXVNIEluU1BpUmlUIC0gZ290bGl1bUBnbWFpbC5jb20KICAgICAgICAgICAgICAgIDwvcD4KICAgICAgICAgICAgPC9kaXY+CiAgICAgICAgPC9kaXY+CiAgICA8L2Rpdj4KPC9kaXY+Cgo8IS0tIE1vZGFsIC0tPgo8ZGl2IGNsYXNzPSJtb2RhbCBmYWRlIiBpZD0ic2hvd1N0YXRpc3RpY3MiIHJvbGU9ImRpYWxvZyI+CiAgICA8ZGl2IGNsYXNzPSJtb2RhbC1kaWFsb2ciPgogICAgICAgIDwhLS0gTW9kYWwgY29udGVudC0tPgogICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWNvbnRlbnQiPgogICAgICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1oZWFkZXIiPgogICAgICAgICAgICAgICAgPGJ1dHRvbiB0eXBlPSJidXR0b24iIGNsYXNzPSJjbG9zZSIgZGF0YS1kaXNtaXNzPSJtb2RhbCI+JnRpbWVzOzwvYnV0dG9uPgogICAgICAgICAgICAgICAgPGg0IGNsYXNzPSJtb2RhbC10aXRsZSI+U3RhdGlzdGljczwvaDQ+CiAgICAgICAgICAgIDwvZGl2PgogICAgICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1ib2R5Ij4KICAgICAgICAgICAgICAgIDxwcmUgaWQ9InN0YXRzIj48L3ByZT4KICAgICAgICAgICAgPC9kaXY+CiAgICAgICAgPC9kaXY+CiAgICA8L2Rpdj4KPC9kaXY+Cgo8IS0tIE1vZGFsIC0tPgo8ZGl2IGNsYXNzPSJtb2RhbCBmYWRlIiBpZD0iZXJyb3JNZXNzYWdlIiByb2xlPSJkaWFsb2ciPgogICAgPGRpdiBjbGFzcz0ibW9kYWwtZGlhbG9nIj4KICAgICAgICA8IS0tIE1vZGFsIGNvbnRlbnQtLT4KICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1jb250ZW50Ij4KICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtaGVhZGVyIj4KICAgICAgICAgICAgICAgIDxidXR0b24gdHlwZT0iYnV0dG9uIiBjbGFzcz0iY2xvc2UiIGRhdGEtZGlzbWlzcz0ibW9kYWwiPiZ0aW1lczs8L2J1dHRvbj4KICAgICAgICAgICAgICAgIDxoNCBjbGFzcz0ibW9kYWwtdGl0bGUiPkVycm9yPC9oND4KICAgICAgICAgICAgPC9kaXY+CiAgICAgICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWJvZHkiIGlkPSJlcnJvci1tZXNzYWdlIj4KICAgICAgICAgICAgPC9kaXY+CiAgICAgICAgPC9kaXY+CiAgICA8L2Rpdj4KPC9kaXY+Cgo8L2JvZHk+CjwvaHRtbD4K"

func HandleWebIndex(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type: text/html", "*")
	content, err := ioutil.ReadFile("index.html")
	if err != nil {
		log.Println("warning: start page not found, return included page")
		val, _ := base64.StdEncoding.DecodeString(htmlData)
		w.Write(val)
		return
	}
	w.Write(content)
}

func HandleWebData(w http.ResponseWriter, req *http.Request) {
	response := []interface{}{}
	for id, server := range LB.config.Servers {
		response = append(
			response, map[string]interface{}{
				"weight": server.Weight,
				"backend": server.Url,
				"type": server.Type,
				"id": id,
		})
	}
	data, err := json.MarshalIndent(response, "", "  ")
	setHttpHeaders(w)
	if err == nil {
		io.WriteString(w, string(data))
	} else {
		setStatus(w, "ERROR")
	}
}

func HandleWebRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
		backend := strings.Replace(r.Form.Get("backend"), `"`, ``, 2)
		removeBackendHandler(w, backend)
	}
}

func (mr *RunCommand) RunWeb() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleWebIndex)
	mux.HandleFunc("/add_backend", HandleAdd)
	mux.HandleFunc("/data.json", HandleWebData)
	mux.HandleFunc("/stats.json", HandleApiStats)
	mux.HandleFunc("/remove_backend", HandleWebRemove)

	n := negroni.New()
	n.Use(helpers.LogMiddleware())
	n.Use(helpers.AuthMiddleware(
		mr.config.LbWebLogin, mr.config.LbWebPassword))

	log.Println("L0xyd Web listen at", mr.config.LbWebAddress)
	n.UseHandler(mux)
	if err := http.ListenAndServe(mr.config.LbWebAddress, n); err != nil {
		log.Errorf("Web server exited with error: %s", err)
	}
}

func (mr *RunCommand) RunApi() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleApiIndex)
	mux.HandleFunc("/stats", HandleApiStats)
	mux.HandleFunc("/status", HandleApiStatus)

	n := negroni.New()
	n.Use(helpers.LogMiddleware())
	n.Use(helpers.AuthMiddleware(
		mr.config.LbApiLogin, mr.config.LbApiPassword))

	log.Println("L0xyd Api listen at", mr.config.ApiAddress)
	n.UseHandler(mux)
	if err := http.ListenAndServe(mr.config.ApiAddress, n); err != nil {
		log.Errorf("Api server exited with error: %s", err)
	}
}

func (mr *RunCommand) RunTLS() {
	if !mr.config.LbSSLEnable { return }
	ss := &http.Server{
		Addr:           mr.config.LbSSLAddress,
		Handler:        LB.stream,
	}
	log.Println("L0xyd Ssl listen at", mr.config.LbSSLAddress)
	if err := ss.ListenAndServeTLS(
		mr.config.LbSSLCert, mr.config.LbSSLKey); err != nil {
		log.Errorf("Ssl server %s exited with error: %s", ss.Addr, err)
	}
}

func (mr *RunCommand) RunHttp() {
	listen := mr.config.LbAddress
	if mr.ListenAddr != "" {
		listen = mr.ListenAddr
	}
	s := &http.Server{
		Addr:           listen,
		Handler:        LB.stream,
	}
	if mr.config.LbWsEnabled {
		// note: websockets not working with stream
		s = &http.Server{
			Addr:           listen,
			Handler:        LB.rl,
		}
	}
	log.Println("L0xyd Http listen at", listen)
	if err := s.ListenAndServe(); err != nil {
		log.Errorf("Server %s exited with error: %s", s.Addr, err)
		os.Exit(255)
	}
}

func (mr *RunCommand) Run() {
	oxyLogger := &helpers.OxyLogger{}
	fwd_logger := forward.Logger(oxyLogger)
	stm_logger := stream.Logger(oxyLogger)
	rb_logger := roundrobin.RebalancerLogger(oxyLogger)

	if mr.config.LbLogFile != "" {
		f, err := os.OpenFile(
			mr.config.LbLogFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		} else {
			fileLogger := utils.NewFileLogger(f, utils.INFO)
			fwd_logger = forward.Logger(fileLogger)
			stm_logger = stream.Logger(fileLogger)
			rb_logger = roundrobin.RebalancerLogger(fileLogger)
			defer f.Close()
		}
	}

	stats := stats.New()
	fwd, _ := forward.New(fwd_logger)

	// Websocket Middleware
	ws_mw, _ := websockets.New(fwd)
	ws := getNextHandler(
		ws_mw, fwd, mr.config.LbWsEnabled, "WebSockets")

	xh_mw, _ := headers.New(ws, mr.config)
	xh := getNextHandler(xh_mw, ws, mr.config.LbEnableXHeader, "X-Header")

	// Tracing Middleware
	trc_log, _ := os.OpenFile(
		mr.config.LbTaceFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	trc_mw, _ := trace.New(xh, trc_log)
	trc := getNextHandler(trc_mw, xh, mr.config.LbEnableTace, "Tracing")

	// Mirroring Middleware
	mrr_mw, _ := mirror.New(trc, mr.config.LbMirroringMethods)
	mrr := getNextHandler(
		mrr_mw, trc, mr.config.LbMirroringEnabled, "Mirroring")

	// Statistics Middleware
	mts_mw, _ := statistics.New(mrr, stats)
	mts := getNextHandler(mts_mw, mrr, mr.config.LbStats, "Statistics")

	// Monitorng Middleware
	mon_mw, _ := monitoring.New(mts, mr.config)
	mon := getNextHandler(
		mon_mw, mts, mr.config.LbMonitorBrokenBackends, "Monitoring")

	lb, _ := roundrobin.New(mon)

	// Rebalancer Middleware
	rb_mw, _ := roundrobin.NewRebalancer(lb, rb_logger)
	rb := getNextHandler(
		rb_mw, lb, mr.config.LbEnableRebalancer, "Rebalancer")

	// Connection Limits Middleware
	extract, _ := utils.NewExtractor(mr.config.LbConnlimitVariable)
	cl_mw, _ := connlimit.New(
		rb, extract, int64(mr.config.LbConnlimitConnections))
	cl := getNextHandler(
		cl_mw, rb, mr.config.LbEnableConnlimit, "Connection Limits")

	// Rate Limits Middleware
	defaultRates := ratelimit.NewRateSet()
	defaultRates.Add(
		time.Duration(mr.config.LbRatelimitPeriodSeconds) * time.Second,
		int64(mr.config.LbRatelimitRequests),
		int64(mr.config.LbRatelimitBurst))
	extractor, _ := utils.NewExtractor(mr.config.LbRatelimitVariable)
	rl_mw, _ := ratelimit.New(cl, extractor, defaultRates)
	rl := getNextHandler(
		rl_mw, cl, mr.config.LbEnableRatelimit, "Rate Limits")

	stream, _ := stream.New(
		rl, stm_logger, stream.Retry(mr.config.LbStreamRetryConditions))

	if mr.config.LbMonitorBrokenBackends {
		go mon_mw.Start(lb)
	}

	for serverName, server := range mr.config.Servers {
		u, err := url.Parse(server.Url)
		if (err == nil && server.Type == "standard") || server.Type == ""  {
			lb.UpsertServer(u, roundrobin.Weight(server.Weight))
			log.Printf(
				"LB: %s (Url=%s, Weight=%d) was added.",
				serverName, server.Url, server.Weight)
		} else if err == nil && server.Type == "mirror" {
			mrr_mw.Add(server.Url)
		}
	}

	LB.lb = lb
	LB.rl = rl
	LB.mirror = mrr_mw
	LB.config = mr.config
	LB.stats = stats
	LB.stream = stream

	go mr.RunApi()
	go mr.RunWeb()
	go mr.RunTLS()
	go mr.RunHttp()
}

func (mr *RunCommand) Start(s service.Service) error {
	if len(mr.WorkingDirectory) > 0 {
		err := os.Chdir(mr.WorkingDirectory)
		if err != nil {
			return err
		}
	}

	err := mr.loadConfig()
	if err != nil {
		panic(err)
	}

	go mr.Run()

	return nil
}

func (mr *RunCommand) Stop(s service.Service) error {
	log.Println("L0xyd: requested service stop")
	mr.saveConfig()
	return nil
}

func (c *RunCommand) Execute(context *cli.Context) {
	svcConfig := &service.Config{
		Name:        c.ServiceName,
		DisplayName: c.ServiceName,
		Description: defaultDescription,
		Arguments:   []string{"run"},
	}

	service, err := service_helpers.New(c, svcConfig)
	if err != nil {
		log.Fatalln(err)
	}

	if c.Syslog {
		logger, err := service.SystemLogger(nil)
		if err == nil {
			log.AddHook(&ServiceLogHook{logger})
		} else {
			log.Errorln(err)
		}
	}

	err = service.Run()
	if err != nil {
		log.Fatalln(err)
	}
}

func init() {
	common.RegisterCommand2("run", "Run Load Balancer", &RunCommand{
		ServiceName: defaultServiceName,
	})
}
