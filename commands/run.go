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
	"github.com/mailgun/oxy/connlimit"
	"github.com/mailgun/oxy/ratelimit"
	"github.com/mailgun/oxy/roundrobin"

	log "github.com/Sirupsen/logrus"
	"github.com/LPgenerator/lpg-load-balancer/common"
	"github.com/LPgenerator/lpg-load-balancer/helpers"
	service "github.com/ayufan/golang-kardianos-service"
	"github.com/LPgenerator/lpg-load-balancer/helpers/service"
	"github.com/LPgenerator/lpg-load-balancer/commands/mirroring"
	"github.com/LPgenerator/lpg-load-balancer/commands/statistics"
	"github.com/LPgenerator/lpg-load-balancer/commands/monitoring"
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
	config           *common.Config
	stats            *stats.Stats
	mirror           *mirror.Mirroring
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
	w.Header().Set("Server", "lpgenerator.ru")
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

const htmlData = "PCFkb2N0eXBlIGh0bWw+Cgo8aHRtbD4KCjxoZWFkPgogICAgPHRpdGxlPldFQiBMQjwvdGl0bGU+CiAgICA8bWV0YSBuYW1lPSJ2aWV3cG9ydCIgY29udGVudD0id2lkdGg9ZGV2aWNlLXdpZHRoIj4KICAgIDxsaW5rIHJlbD0ic3R5bGVzaGVldCIgaHJlZj0iaHR0cHM6Ly9uZXRkbmEuYm9vdHN0cmFwY2RuLmNvbS9ib290c3dhdGNoLzMuMC4wL2pvdXJuYWwvYm9vdHN0cmFwLm1pbi5jc3MiPgogICAgPGxpbmsgcmVsPSJzdHlsZXNoZWV0IiB0eXBlPSJ0ZXh0L2NzcyIgbWVkaWE9InNjcmVlbiIKICAgICAgICAgIGhyZWY9Imh0dHA6Ly93d3cuZ3VyaWRkby5uZXQvZGVtby9jc3MvdHJpcmFuZC91aS5qcWdyaWQtYm9vdHN0cmFwLmNzcyI+CiAgICA8c2NyaXB0IHR5cGU9InRleHQvamF2YXNjcmlwdCIgc3JjPSJodHRwczovL2FqYXguZ29vZ2xlYXBpcy5jb20vYWpheC9saWJzL2pxdWVyeS8yLjAuMy9qcXVlcnkubWluLmpzIj48L3NjcmlwdD4KICAgIDxzY3JpcHQgdHlwZT0idGV4dC9qYXZhc2NyaXB0IiBzcmM9Imh0dHBzOi8vbmV0ZG5hLmJvb3RzdHJhcGNkbi5jb20vYm9vdHN0cmFwLzMuMy40L2pzL2Jvb3RzdHJhcC5taW4uanMiPjwvc2NyaXB0PgogICAgPHNjcmlwdCB0eXBlPSJ0ZXh0L2phdmFzY3JpcHQiIHNyYz0iaHR0cDovL3d3dy5ndXJpZGRvLm5ldC9kZW1vL2pzL3RyaXJhbmQvanF1ZXJ5LmpxR3JpZC5taW4uanMiPjwvc2NyaXB0PgogICAgPHNjcmlwdCB0eXBlPSJ0ZXh0L2phdmFzY3JpcHQiIHNyYz0iaHR0cDovL3d3dy5ndXJpZGRvLm5ldC9kZW1vL2pzL3RyaXJhbmQvaTE4bi9ncmlkLmxvY2FsZS1lbi5qcyI+PC9zY3JpcHQ+CiAgICA8bGluayByZWw9InN0eWxlc2hlZXQiIGhyZWY9Ii8vY29kZS5qcXVlcnkuY29tL3VpLzEuMTEuNC90aGVtZXMvc21vb3RobmVzcy9qcXVlcnktdWkuY3NzIj4KICAgIDxzY3JpcHQgc3JjPSJodHRwOi8vY29kZS5qcXVlcnkuY29tL3VpLzEuMTEuNC9qcXVlcnktdWkuanMiPjwvc2NyaXB0PgogICAgPHN0eWxlIHR5cGU9InRleHQvY3NzIj4KICAgICAgICBib2R5IHsKICAgICAgICAgICAgcGFkZGluZy10b3A6IDUwcHg7CiAgICAgICAgICAgIHBhZGRpbmctYm90dG9tOiAyMHB4OwogICAgICAgIH0KICAgIDwvc3R5bGU+CiAgICA8c2NyaXB0IHR5cGU9InRleHQvamF2YXNjcmlwdCI+CiAgICAgICAgZnVuY3Rpb24gVXBkYXRlVGFibGUoKSB7CiAgICAgICAgICAgICQoIiNqcUdyaWQiKQogICAgICAgICAgICAgICAgLmpxR3JpZCh7CiAgICAgICAgICAgICAgICAgICAgdXJsOiAnL2RhdGEuanNvbicsCiAgICAgICAgICAgICAgICAgICAgbXR5cGU6ICJHRVQiLAogICAgICAgICAgICAgICAgICAgIGFqYXhTdWJncmlkT3B0aW9uczogewogICAgICAgICAgICAgICAgICAgICAgICBhc3luYzogZmFsc2UKICAgICAgICAgICAgICAgICAgICB9LAogICAgICAgICAgICAgICAgICAgIHN0eWxlVUk6ICdCb290c3RyYXAnLAogICAgICAgICAgICAgICAgICAgIGRhdGF0eXBlOiAianNvbiIsCiAgICAgICAgICAgICAgICAgICAgY29sTW9kZWw6IFt7CiAgICAgICAgICAgICAgICAgICAgICAgIGxhYmVsOiAnIycsCiAgICAgICAgICAgICAgICAgICAgICAgIG5hbWU6ICdpZCcsCiAgICAgICAgICAgICAgICAgICAgICAgIHdpZHRoOiA1CiAgICAgICAgICAgICAgICAgICAgfSwgewogICAgICAgICAgICAgICAgICAgICAgICBsYWJlbDogJ0JhY2tlbmQnLAogICAgICAgICAgICAgICAgICAgICAgICBuYW1lOiAnYmFja2VuZCcsCiAgICAgICAgICAgICAgICAgICAgICAgIGtleTogdHJ1ZSwKICAgICAgICAgICAgICAgICAgICAgICAgd2lkdGg6IDE1CiAgICAgICAgICAgICAgICAgICAgfSwgewogICAgICAgICAgICAgICAgICAgICAgICBsYWJlbDogJ1dlaWdodCcsCiAgICAgICAgICAgICAgICAgICAgICAgIG5hbWU6ICd3ZWlnaHQnLAogICAgICAgICAgICAgICAgICAgICAgICB3aWR0aDogMjAKICAgICAgICAgICAgICAgICAgICB9LCB7CiAgICAgICAgICAgICAgICAgICAgICAgIGxhYmVsOiAnVHlwZScsCiAgICAgICAgICAgICAgICAgICAgICAgIG5hbWU6ICd0eXBlJywKICAgICAgICAgICAgICAgICAgICAgICAgd2lkdGg6IDIwCiAgICAgICAgICAgICAgICAgICAgfV0sCiAgICAgICAgICAgICAgICAgICAgdmlld3JlY29yZHM6IHRydWUsCiAgICAgICAgICAgICAgICAgICAgcm93TnVtOiAyMCwKICAgICAgICAgICAgICAgICAgICBwYWdlcjogIiNqcUdyaWRQYWdlciIKICAgICAgICAgICAgICAgIH0pCiAgICAgICAgfQoKICAgICAgICBmdW5jdGlvbiBGaXhUYWJsZSgpIHsKICAgICAgICAgICAgJC5leHRlbmQoJC5qZ3JpZC5hamF4T3B0aW9ucywgewogICAgICAgICAgICAgICAgYXN5bmM6IGZhbHNlCiAgICAgICAgICAgIH0pOwogICAgICAgICAgICB2YXIgZ3JpZCA9ICQoIiNqcUdyaWQiKTsKICAgICAgICAgICAgZ3JpZC5zZXRHcmlkV2lkdGgoJCh3aW5kb3cpLndpZHRoKCkgLSA1KTsKICAgICAgICAgICAgZ3JpZC5zZXRHcmlkSGVpZ2h0KCQod2luZG93KS5oZWlnaHQoKSk7CiAgICAgICAgICAgICQod2luZG93KS5iaW5kKCdyZXNpemUnLCBmdW5jdGlvbigpIHsKICAgICAgICAgICAgICAgIHZhciBqcV9ncmlkID0gJCgiI2pxR3JpZCIpOwogICAgICAgICAgICAgICAganFfZ3JpZC5zZXRHcmlkV2lkdGgoJCh3aW5kb3cpLndpZHRoKCkgLSA1KTsKICAgICAgICAgICAgICAgIGpxX2dyaWQuc2V0R3JpZEhlaWdodCgkKHdpbmRvdykuaGVpZ2h0KCkpOwogICAgICAgICAgICB9KTsKICAgICAgICB9CgogICAgICAgIGZ1bmN0aW9uIFZhbGlkYXRlQmFja2VuZChiYWNrZW5kKSB7CiAgICAgICAgICAgIHJldHVybiBiYWNrZW5kLm1hdGNoKC9eKD86WzAtOV17MSwzfVwuKXszfVswLTldezEsM306WzAtOV17Miw1fSQvKTsKICAgICAgICB9CgogICAgICAgIGZ1bmN0aW9uIEFkZEJhY2tlbmQoKSB7CiAgICAgICAgICAgIHZhciB1cmwgPSAkKCIjdXJsX2lkIikudmFsKCkucmVwbGFjZSgiaHR0cDovLyIsICIiKTsKICAgICAgICAgICAgdmFyIHdlaWdodCA9ICQoIiN3ZWlnaHRfaWQiKS52YWwoKTsKCiAgICAgICAgICAgIGlmICghVmFsaWRhdGVCYWNrZW5kKHVybCkpIHsKICAgICAgICAgICAgICAgIHJldHVybiBzaG93RXJyb3IoIkludmFsaWQgYmFja2VuZCEiKQogICAgICAgICAgICB9CgogICAgICAgICAgICBpZiAoIXdlaWdodC5tYXRjaCgvXlswLTldezEsMn0kLykpIHsKICAgICAgICAgICAgICAgIHJldHVybiBzaG93RXJyb3IoIkludmFsaWQgd2VpZ2h0ISIpCiAgICAgICAgICAgIH0KCiAgICAgICAgICAgIHZhciByZXEgPSB7CiAgICAgICAgICAgICAgICB3ZWlnaHQ6IHBhcnNlSW50KHdlaWdodCksCiAgICAgICAgICAgICAgICB1cmw6IHVybCwKICAgICAgICAgICAgICAgIHR5cGU6ICQoIiN0eXBlX2lkIikudmFsKCkKICAgICAgICAgICAgfTsKCiAgICAgICAgICAgICQuYWpheCh7CiAgICAgICAgICAgICAgICB1cmw6ICIvYWRkX2JhY2tlbmQiLAogICAgICAgICAgICAgICAgdHlwZTogIlBPU1QiLAogICAgICAgICAgICAgICAgZGF0YTogcmVxLAogICAgICAgICAgICAgICAgZGF0YVR5cGU6ICJqc29uIgogICAgICAgICAgICB9KS5kb25lKGZ1bmN0aW9uKGRhdGEpIHsKICAgICAgICAgICAgICAgIGlmICh0eXBlb2YgKGRhdGEuc3RhdHVzKSA9PSAidW5kZWZpbmVkIiB8fCBkYXRhLnN0YXR1cyAhPSAiT0siKSB7CiAgICAgICAgICAgICAgICAgICAgc2hvd0Vycm9yKCJFcnJvciBvY2N1cnJlZCAuLi4iKTsKICAgICAgICAgICAgICAgIH0gZWxzZSB7CiAgICAgICAgICAgICAgICAgICAgVXBkYXRlRGF0YSgpOwogICAgICAgICAgICAgICAgfQogICAgICAgICAgICB9KS5lcnJvcihmdW5jdGlvbigpIHsKICAgICAgICAgICAgICAgIHNob3dFcnJvcigiQ29ubmVjdGlvbiBlcnJvciAuLi4iKTsKICAgICAgICAgICAgfSk7CiAgICAgICAgfQoKICAgICAgICBmdW5jdGlvbiBSZW1vdmVCYWNrZW5kKCkgewogICAgICAgICAgICB2YXIgZ3JpZCA9ICQoIiNqcUdyaWQiKTsKICAgICAgICAgICAgdmFyIHJvd0tleSA9IGdyaWQuanFHcmlkKCdnZXRHcmlkUGFyYW0nLCAic2Vscm93Iik7CgogICAgICAgICAgICBpZiAocm93S2V5ID09IG51bGwpIHsKICAgICAgICAgICAgICAgIHJldHVybiBzaG93RXJyb3IoIkJhY2tlbmQgaXMgbm90IHNlbGVjdGVkISIpCiAgICAgICAgICAgIH0KCiAgICAgICAgICAgIHZhciB1cmwgPSByb3dLZXkucmVwbGFjZSgiaHR0cDovLyIsICIiKTsKICAgICAgICAgICAgaWYgKCFWYWxpZGF0ZUJhY2tlbmQodXJsKSkgewogICAgICAgICAgICAgICAgcmV0dXJuIHNob3dFcnJvcigiSW52YWxpZCBiYWNrZW5kISIpCiAgICAgICAgICAgIH0KCiAgICAgICAgICAgICQuYWpheCh7CiAgICAgICAgICAgICAgICB1cmw6ICIvcmVtb3ZlX2JhY2tlbmQiLAogICAgICAgICAgICAgICAgdHlwZTogIlBPU1QiLAogICAgICAgICAgICAgICAgZGF0YToge2JhY2tlbmQ6IHVybH0sCiAgICAgICAgICAgICAgICBkYXRhVHlwZTogImpzb24iCiAgICAgICAgICAgIH0pLmRvbmUoZnVuY3Rpb24oZGF0YSkgewogICAgICAgICAgICAgICAgaWYgKHR5cGVvZiAoZGF0YS5zdGF0dXMpID09ICJ1bmRlZmluZWQiIHx8IGRhdGEuc3RhdHVzICE9ICJPSyIpIHsKICAgICAgICAgICAgICAgICAgICBzaG93RXJyb3IoIkVycm9yIG9jY3VycmVkIC4uLiIpOwogICAgICAgICAgICAgICAgfSBlbHNlIHsKICAgICAgICAgICAgICAgICAgICBVcGRhdGVEYXRhKCk7CiAgICAgICAgICAgICAgICB9CiAgICAgICAgICAgIH0pLmVycm9yKGZ1bmN0aW9uKCkgewogICAgICAgICAgICAgICAgc2hvd0Vycm9yKCJDb25uZWN0aW9uIGVycm9yIC4uLiIpOwogICAgICAgICAgICB9KTsKICAgICAgICB9CgogICAgICAgIGZ1bmN0aW9uIFVwZGF0ZURhdGEoKSB7CiAgICAgICAgICAgIHZhciBncmlkID0gJCgiI2pxR3JpZCIpOwogICAgICAgICAgICB2YXIgcm93S2V5ID0gZ3JpZC5qcUdyaWQoJ2dldEdyaWRQYXJhbScsICJzZWxyb3ciKTsKICAgICAgICAgICAgZ3JpZC50cmlnZ2VyKCJyZWxvYWRHcmlkIik7CiAgICAgICAgICAgIGlmKHJvd0tleSkgewogICAgICAgICAgICAgICAgZ3JpZC5qcUdyaWQoInJlc2V0U2VsZWN0aW9uIik7CiAgICAgICAgICAgICAgICBncmlkLmpxR3JpZCgnc2V0U2VsZWN0aW9uJywgcm93S2V5KTsKICAgICAgICAgICAgfQogICAgICAgIH0KCiAgICAgICAgZnVuY3Rpb24gVXBkYXRlU3RhdHMoKSB7CiAgICAgICAgICAgICQuYWpheCh7CiAgICAgICAgICAgICAgICB1cmw6ICIvc3RhdHMuanNvbiIsCiAgICAgICAgICAgICAgICB0eXBlOiAiR0VUIiwKICAgICAgICAgICAgICAgIGRhdGFUeXBlOiAidGV4dCIKICAgICAgICAgICAgfSkuZG9uZShmdW5jdGlvbihkYXRhKSB7CiAgICAgICAgICAgICAgICAkKCcjc3RhdHMnKS50ZXh0KGRhdGEpOwogICAgICAgICAgICB9KTsKCiAgICAgICAgfQoKICAgICAgICBmdW5jdGlvbiBzaG93RXJyb3IobWVzc2FnZSkgewogICAgICAgICAgICAkKCIjZXJyb3JNZXNzYWdlIikubW9kYWwoInNob3ciKTsKICAgICAgICAgICAgJCgiI2Vycm9yLW1lc3NhZ2UiKS5odG1sKG1lc3NhZ2UpOwogICAgICAgIH0KCiAgICAgICAgZnVuY3Rpb24gT25Mb2FkKCkgewogICAgICAgICAgICBVcGRhdGVUYWJsZSgpOwogICAgICAgICAgICBGaXhUYWJsZSgpOwogICAgICAgICAgICBVcGRhdGVTdGF0cygpOwogICAgICAgICAgICBzZXRJbnRlcnZhbChVcGRhdGVEYXRhLCAxNTAwMCk7CiAgICAgICAgICAgIHNldEludGVydmFsKFVwZGF0ZVN0YXRzLCA1MDAwKTsKICAgICAgICB9CiAgICA8L3NjcmlwdD4KPC9oZWFkPgoKPGJvZHkgb25sb2FkPSJPbkxvYWQoKSI+CjxkaXYgY2xhc3M9Im5hdmJhciBuYXZiYXItaW52ZXJzZSBuYXZiYXItZml4ZWQtdG9wIj4KICAgIDxkaXYgY2xhc3M9ImNvbnRhaW5lciI+CiAgICAgICAgPGRpdiBjbGFzcz0ibmF2YmFyLWhlYWRlciI+CiAgICAgICAgICAgIDxidXR0b24gdHlwZT0iYnV0dG9uIiBjbGFzcz0ibmF2YmFyLXRvZ2dsZSIgZGF0YS10b2dnbGU9ImNvbGxhcHNlIiBkYXRhLXRhcmdldD0iLm5hdmJhci1jb2xsYXBzZSI+CiAgICAgICAgICAgICAgICA8c3BhbiBjbGFzcz0iaWNvbi1iYXIiPjwvc3Bhbj48c3BhbiBjbGFzcz0iaWNvbi1iYXIiPjwvc3Bhbj48c3BhbiBjbGFzcz0iaWNvbi1iYXIiPjwvc3Bhbj4KICAgICAgICAgICAgPC9idXR0b24+CiAgICAgICAgICAgIDxhIGNsYXNzPSJuYXZiYXItYnJhbmQiIGhyZWY9IiMiPkxCIFdFQjwvYT4KICAgICAgICA8L2Rpdj4KICAgICAgICA8ZGl2IGNsYXNzPSJuYXZiYXItY29sbGFwc2UgY29sbGFwc2UiPgogICAgICAgICAgICA8dWwgY2xhc3M9Im5hdiBuYXZiYXItbmF2Ij4KICAgICAgICAgICAgICAgIDxsaSBjbGFzcz0iZHJvcGRvd24iPgogICAgICAgICAgICAgICAgICAgIDxhIGhyZWY9IiMiIGNsYXNzPSJkcm9wZG93bi10b2dnbGUiIGRhdGEtdG9nZ2xlPSJkcm9wZG93biI+QmFja2VuZHMgPGIgY2xhc3M9ImNhcmV0Ij48L2I+PC9hPgogICAgICAgICAgICAgICAgICAgIDx1bCBjbGFzcz0iZHJvcGRvd24tbWVudSI+CiAgICAgICAgICAgICAgICAgICAgICAgIDxsaT4KICAgICAgICAgICAgICAgICAgICAgICAgICAgIDxhIGRhdGEtdG9nZ2xlPSJtb2RhbCIgZGF0YS10YXJnZXQ9IiNhZGRCYWNrZW5kIj5BZGQgQmFja2VuZDwvYT4KICAgICAgICAgICAgICAgICAgICAgICAgPC9saT4KICAgICAgICAgICAgICAgICAgICAgICAgPGxpIG9uY2xpY2s9IlJlbW92ZUJhY2tlbmQoKSI+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICA8YSBocmVmPSIjIj5EZWxldGUgQmFja2VuZDwvYT4KICAgICAgICAgICAgICAgICAgICAgICAgPC9saT4KICAgICAgICAgICAgICAgICAgICA8L3VsPgogICAgICAgICAgICAgICAgPC9saT4KICAgICAgICAgICAgICAgIDxsaT4KICAgICAgICAgICAgICAgICAgICA8YSBkYXRhLXRvZ2dsZT0ibW9kYWwiIGRhdGEtdGFyZ2V0PSIjc2hvd1N0YXRpc3RpY3MiPlN0YXRpc3RpY3M8L2E+CiAgICAgICAgICAgICAgICA8L2xpPgogICAgICAgICAgICAgICAgPGxpPgogICAgICAgICAgICAgICAgICAgIDxhIGRhdGEtdG9nZ2xlPSJtb2RhbCIgZGF0YS10YXJnZXQ9IiNhYm91dFdpbmRvdyI+QWJvdXQ8L2E+CiAgICAgICAgICAgICAgICA8L2xpPgogICAgICAgICAgICA8L3VsPgogICAgICAgIDwvZGl2PgogICAgICAgIDwhLS0vLm5hdmJhci1jb2xsYXBzZSAtLT4KICAgIDwvZGl2Pgo8L2Rpdj4KPC9QPgo8dGFibGUgaWQ9ImpxR3JpZCI+PC90YWJsZT4KCjwhLS0gTW9kYWwgLS0+CjxkaXYgY2xhc3M9Im1vZGFsIGZhZGUiIGlkPSJhZGRCYWNrZW5kIiByb2xlPSJkaWFsb2ciPgogICAgPGRpdiBjbGFzcz0ibW9kYWwtZGlhbG9nIj4KICAgICAgICA8IS0tIE1vZGFsIGNvbnRlbnQtLT4KICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1jb250ZW50Ij4KICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtaGVhZGVyIj4KICAgICAgICAgICAgICAgIDxidXR0b24gdHlwZT0iYnV0dG9uIiBjbGFzcz0iY2xvc2UiIGRhdGEtZGlzbWlzcz0ibW9kYWwiPiZ0aW1lczs8L2J1dHRvbj4KICAgICAgICAgICAgICAgIDxoNCBjbGFzcz0ibW9kYWwtdGl0bGUiPkJhY2tlbmQ8L2g0PgogICAgICAgICAgICA8L2Rpdj4KICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtYm9keSI+CiAgICAgICAgICAgICAgICA8ZGl2IGNsYXNzPSJmb3JtLWdyb3VwIj4KCiAgICAgICAgICAgICAgICAgICAgPGxhYmVsIGNsYXNzPSJjb250cm9sLWxhYmVsIj5Vcmw8L2xhYmVsPgogICAgICAgICAgICAgICAgICAgIDxkaXYgY2xhc3M9ImNvbnRyb2xzIj4KICAgICAgICAgICAgICAgICAgICAgICAgPGlucHV0IHR5cGU9InRleHQiIGlkPSJ1cmxfaWQiIGNsYXNzPSJmb3JtLWNvbnRyb2wiIHZhbHVlPSJodHRwOi8vMTI3LjAuMC4xOjgwODEiPgogICAgICAgICAgICAgICAgICAgIDwvZGl2PgoKICAgICAgICAgICAgICAgICAgICA8bGFiZWwgY2xhc3M9ImNvbnRyb2wtbGFiZWwiPldlaWdodDwvbGFiZWw+CiAgICAgICAgICAgICAgICAgICAgPGRpdiBjbGFzcz0iY29udHJvbHMiPgogICAgICAgICAgICAgICAgICAgICAgICA8aW5wdXQgdHlwZT0idGV4dCIgaWQ9IndlaWdodF9pZCIgY2xhc3M9ImZvcm0tY29udHJvbCIgdmFsdWU9IjEiPgogICAgICAgICAgICAgICAgICAgIDwvZGl2PgoKICAgICAgICAgICAgICAgICAgICA8bGFiZWwgY2xhc3M9ImNvbnRyb2wtbGFiZWwiPlR5cGU8L2xhYmVsPgogICAgICAgICAgICAgICAgICAgIDxkaXYgY2xhc3M9ImNvbnRyb2xzIj4KICAgICAgICAgICAgICAgICAgICAgICAgPHNlbGVjdCBjbGFzcz0iZm9ybS1jb250cm9sIiBpZD0idHlwZV9pZCI+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICA8b3B0aW9uPnN0YW5kYXJkPC9vcHRpb24+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICA8b3B0aW9uPm1pcnJvcjwvb3B0aW9uPgogICAgICAgICAgICAgICAgICAgICAgICAgICAgPG9wdGlvbj5kb3duPC9vcHRpb24+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICA8b3B0aW9uPmJhY2t1cDwvb3B0aW9uPgogICAgICAgICAgICAgICAgICAgICAgICA8L3NlbGVjdD4KICAgICAgICAgICAgICAgICAgICA8L2Rpdj4KCiAgICAgICAgICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtZm9vdGVyIj4KICAgICAgICAgICAgICAgICAgICAgICAgPGEgY2xhc3M9ImJ0biBidG4tcHJpbWFyeSIgb25jbGljaz0iQWRkQmFja2VuZCgpIiBkYXRhLWRpc21pc3M9Im1vZGFsIj5BZGQgYmFja2VuZDwvYT4KICAgICAgICAgICAgICAgICAgICA8L2Rpdj4KICAgICAgICAgICAgICAgIDwvZGl2PgogICAgICAgICAgICA8L2Rpdj4KICAgICAgICA8L2Rpdj4KICAgIDwvZGl2Pgo8L2Rpdj4KCjwhLS0gTW9kYWwgLS0+CjxkaXYgY2xhc3M9Im1vZGFsIGZhZGUiIGlkPSJhYm91dFdpbmRvdyIgcm9sZT0iZGlhbG9nIj4KICAgIDxkaXYgY2xhc3M9Im1vZGFsLWRpYWxvZyI+CiAgICAgICAgPCEtLSBNb2RhbCBjb250ZW50LS0+CiAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtY29udGVudCI+CiAgICAgICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWhlYWRlciI+CiAgICAgICAgICAgICAgICA8YnV0dG9uIHR5cGU9ImJ1dHRvbiIgY2xhc3M9ImNsb3NlIiBkYXRhLWRpc21pc3M9Im1vZGFsIj4mdGltZXM7PC9idXR0b24+CiAgICAgICAgICAgICAgICA8aDQgY2xhc3M9Im1vZGFsLXRpdGxlIj5BYm91dDwvaDQ+CiAgICAgICAgICAgIDwvZGl2PgogICAgICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1ib2R5Ij4KICAgICAgICAgICAgICAgIDxwPgogICAgICAgICAgICAgICAgICAgIDxzdHJvbmc+TkFNRTo8L3N0cm9uZz48L2JyPgogICAgICAgICAgICAgICAgICAgICZuYnNwOyZuYnNwOyZuYnNwOyZuYnNwO2xwZy1sb2FkLWJhbGFuY2VyIC0gU2ltcGxlIGxvYWQgYmFsYW5jZXIgd2l0aCBIdHRwIEFQSS4KICAgICAgICAgICAgICAgIDwvcD4KICAgICAgICAgICAgICAgIDxwPgogICAgICAgICAgICAgICAgICAgIDxzdHJvbmc+QVVUSE9SKFMpOjwvc3Ryb25nPjwvYnI+CiAgICAgICAgICAgICAgICAgICAgJm5ic3A7Jm5ic3A7Jm5ic3A7Jm5ic3A7R29UTGl1TSBJblNQaVJpVCA8Z290bGl1bUBnbWFpbC5jb20+CiAgICAgICAgICAgICAgICA8L3A+CiAgICAgICAgICAgIDwvZGl2PgogICAgICAgIDwvZGl2PgogICAgPC9kaXY+CjwvZGl2PgoKPCEtLSBNb2RhbCAtLT4KPGRpdiBjbGFzcz0ibW9kYWwgZmFkZSIgaWQ9InNob3dTdGF0aXN0aWNzIiByb2xlPSJkaWFsb2ciPgogICAgPGRpdiBjbGFzcz0ibW9kYWwtZGlhbG9nIj4KICAgICAgICA8IS0tIE1vZGFsIGNvbnRlbnQtLT4KICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1jb250ZW50Ij4KICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtaGVhZGVyIj4KICAgICAgICAgICAgICAgIDxidXR0b24gdHlwZT0iYnV0dG9uIiBjbGFzcz0iY2xvc2UiIGRhdGEtZGlzbWlzcz0ibW9kYWwiPiZ0aW1lczs8L2J1dHRvbj4KICAgICAgICAgICAgICAgIDxoNCBjbGFzcz0ibW9kYWwtdGl0bGUiPlN0YXRpc3RpY3M8L2g0PgogICAgICAgICAgICA8L2Rpdj4KICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtYm9keSI+CiAgICAgICAgICAgICAgICA8cHJlIGlkPSJzdGF0cyI+PC9wcmU+CiAgICAgICAgICAgIDwvZGl2PgogICAgICAgIDwvZGl2PgogICAgPC9kaXY+CjwvZGl2PgoKPCEtLSBNb2RhbCAtLT4KPGRpdiBjbGFzcz0ibW9kYWwgZmFkZSIgaWQ9ImVycm9yTWVzc2FnZSIgcm9sZT0iZGlhbG9nIj4KICAgIDxkaXYgY2xhc3M9Im1vZGFsLWRpYWxvZyI+CiAgICAgICAgPCEtLSBNb2RhbCBjb250ZW50LS0+CiAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtY29udGVudCI+CiAgICAgICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWhlYWRlciI+CiAgICAgICAgICAgICAgICA8YnV0dG9uIHR5cGU9ImJ1dHRvbiIgY2xhc3M9ImNsb3NlIiBkYXRhLWRpc21pc3M9Im1vZGFsIj4mdGltZXM7PC9idXR0b24+CiAgICAgICAgICAgICAgICA8aDQgY2xhc3M9Im1vZGFsLXRpdGxlIj5FcnJvcjwvaDQ+CiAgICAgICAgICAgIDwvZGl2PgogICAgICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1ib2R5IiBpZD0iZXJyb3ItbWVzc2FnZSI+CiAgICAgICAgICAgIDwvZGl2PgogICAgICAgIDwvZGl2PgogICAgPC9kaXY+CjwvZGl2PgoKPC9ib2R5Pgo8L2h0bWw+Cg=="

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
				"id": id,
				"backend": server.Url,
				"weight": server.Weight,
				"type": server.Type,
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


func (mr *RunCommand) Run() {
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc(
			"/", helpers.BasicAuth(
			helpers.LogRequests(HandleApiIndex),
			mr.config.LbApiLogin, mr.config.LbApiPassword))
		mux.HandleFunc(
			"/stats", helpers.BasicAuth(
			helpers.LogRequests(HandleApiStats),
			mr.config.LbApiLogin, mr.config.LbApiPassword))
		mux.HandleFunc(
			"/status", helpers.BasicAuth(
			helpers.LogRequests(HandleApiStatus),
			mr.config.LbApiLogin, mr.config.LbApiPassword))
		log.Println("LB API listen at", mr.config.ApiAddress)
		if err := http.ListenAndServe(mr.config.ApiAddress, mux); err != nil {
			log.Errorf("Api server exited with error: %s", err)
		}
	}()

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc(
			"/", helpers.BasicAuth(
			helpers.LogRequests(HandleWebIndex),
			mr.config.LbWebLogin, mr.config.LbWebPassword))
		mux.HandleFunc(
			"/data.json", helpers.BasicAuth(
			helpers.LogRequests(HandleWebData),
			mr.config.LbWebLogin, mr.config.LbWebPassword))
		mux.HandleFunc(
			"/remove_backend", helpers.BasicAuth(
			helpers.LogRequests(HandleWebRemove),
			mr.config.LbWebLogin, mr.config.LbWebPassword))
		mux.HandleFunc(
			"/add_backend", helpers.BasicAuth(
			helpers.LogRequests(HandleAdd),
			mr.config.LbWebLogin, mr.config.LbWebPassword))
		mux.HandleFunc(
			"/stats.json", helpers.BasicAuth(
			helpers.LogRequests(HandleApiStats),
			mr.config.LbWebLogin, mr.config.LbWebPassword))
		log.Println("LB Web listen at", mr.config.LbWebAddress)
		if err := http.ListenAndServe(mr.config.LbWebAddress, mux); err != nil {
			log.Errorf("Web server exited with error: %s", err)
		}
	}()

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

	// Tracing Middleware
	trc_log, _ := os.OpenFile(
		mr.config.LbTaceFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	trc_mw, _ := trace.New(fwd, trc_log)
	trc := getNextHandler(trc_mw, fwd, mr.config.LbEnableTace, "Tracing")

	// Mirroring Middleware
	mrr_mw, _ := mirror.New(trc, mr.config.LbMirroringMethods)
	mrr := getNextHandler(mrr_mw, trc, mr.config.LbMirroringEnabled, "Mirroring")

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

	//todo: Memetrics Middleware

	LB.lb = lb
	LB.mirror = mrr_mw
	LB.config = mr.config
	LB.stats = stats

	if mr.config.LbMonitorBrokenBackends {
		go mon_mw.Start(lb)
	}

	listen := mr.config.LbAddress
	if mr.ListenAddr != "" {
		listen = mr.ListenAddr
	}

	s := &http.Server{
		Addr:           listen,
		Handler:        stream,
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

	log.Println("LB listen at", listen)
	if err := s.ListenAndServe(); err != nil {
		log.Errorf("Server %s exited with error: %s", s.Addr, err)
		os.Exit(255)
	}
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
	log.Println("LB: requested service stop")
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
