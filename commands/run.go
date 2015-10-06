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

const htmlData = "PCFkb2N0eXBlIGh0bWw+Cgo8aHRtbD4KCjxoZWFkPgogICAgPHRpdGxlPldFQiBMQjwvdGl0bGU+CiAgICA8bWV0YSBuYW1lPSJ2aWV3cG9ydCIgY29udGVudD0id2lkdGg9ZGV2aWNlLXdpZHRoIj4KICAgIDxsaW5rIHJlbD0ic3R5bGVzaGVldCIgaHJlZj0iaHR0cHM6Ly9uZXRkbmEuYm9vdHN0cmFwY2RuLmNvbS9ib290c3dhdGNoLzMuMC4wL2pvdXJuYWwvYm9vdHN0cmFwLm1pbi5jc3MiPgogICAgPGxpbmsgcmVsPSJzdHlsZXNoZWV0IiB0eXBlPSJ0ZXh0L2NzcyIgbWVkaWE9InNjcmVlbiIKICAgICAgICAgIGhyZWY9Imh0dHA6Ly93d3cuZ3VyaWRkby5uZXQvZGVtby9jc3MvdHJpcmFuZC91aS5qcWdyaWQtYm9vdHN0cmFwLmNzcyI+CiAgICA8c2NyaXB0IHR5cGU9InRleHQvamF2YXNjcmlwdCIgc3JjPSJodHRwczovL2FqYXguZ29vZ2xlYXBpcy5jb20vYWpheC9saWJzL2pxdWVyeS8yLjAuMy9qcXVlcnkubWluLmpzIj48L3NjcmlwdD4KICAgIDxzY3JpcHQgdHlwZT0idGV4dC9qYXZhc2NyaXB0IiBzcmM9Imh0dHBzOi8vbmV0ZG5hLmJvb3RzdHJhcGNkbi5jb20vYm9vdHN0cmFwLzMuMy40L2pzL2Jvb3RzdHJhcC5taW4uanMiPjwvc2NyaXB0PgogICAgPHNjcmlwdCB0eXBlPSJ0ZXh0L2phdmFzY3JpcHQiIHNyYz0iaHR0cDovL3d3dy5ndXJpZGRvLm5ldC9kZW1vL2pzL3RyaXJhbmQvanF1ZXJ5LmpxR3JpZC5taW4uanMiPjwvc2NyaXB0PgogICAgPHNjcmlwdCB0eXBlPSJ0ZXh0L2phdmFzY3JpcHQiIHNyYz0iaHR0cDovL3d3dy5ndXJpZGRvLm5ldC9kZW1vL2pzL3RyaXJhbmQvaTE4bi9ncmlkLmxvY2FsZS1lbi5qcyI+PC9zY3JpcHQ+CiAgICA8bGluayByZWw9InN0eWxlc2hlZXQiIGhyZWY9Ii8vY29kZS5qcXVlcnkuY29tL3VpLzEuMTEuNC90aGVtZXMvc21vb3RobmVzcy9qcXVlcnktdWkuY3NzIj4KICAgIDxzY3JpcHQgc3JjPSJodHRwOi8vY29kZS5qcXVlcnkuY29tL3VpLzEuMTEuNC9qcXVlcnktdWkuanMiPjwvc2NyaXB0PgogICAgPHN0eWxlIHR5cGU9InRleHQvY3NzIj4KICAgICAgICBib2R5IHsKICAgICAgICAgICAgcGFkZGluZy10b3A6IDUwcHg7CiAgICAgICAgICAgIHBhZGRpbmctYm90dG9tOiAyMHB4OwogICAgICAgIH0KICAgIDwvc3R5bGU+CiAgICA8c2NyaXB0IHR5cGU9InRleHQvamF2YXNjcmlwdCI+CiAgICAgICAgZnVuY3Rpb24gQWRkQmFja2VuZCgpIHsKICAgICAgICAgICAgdmFyIHJlcSA9IHsKICAgICAgICAgICAgICAgIHdlaWdodDogcGFyc2VJbnQoJCgiI3dlaWdodF9pZCIpLnZhbCgpKSwKICAgICAgICAgICAgICAgIHVybDogJCgiI3VybF9pZCIpLnZhbCgpLnJlcGxhY2UoImh0dHA6Ly8iLCAiIiksCiAgICAgICAgICAgICAgICB0eXBlOiAkKCIjdHlwZV9pZCIpLnZhbCgpCiAgICAgICAgICAgIH07CiAgICAgICAgICAgIC8vYWxlcnQoSlNPTi5zdHJpbmdpZnkocmVxKSk7CiAgICAgICAgICAgICQuYWpheCh7CiAgICAgICAgICAgICAgICB1cmw6ICIvYWRkX2JhY2tlbmQiLAogICAgICAgICAgICAgICAgdHlwZTogIlBPU1QiLAogICAgICAgICAgICAgICAgZGF0YTogcmVxLAogICAgICAgICAgICAgICAgZGF0YVR5cGU6ICJ0ZXh0IgogICAgICAgICAgICB9KS5kb25lKGZ1bmN0aW9uKCkgewogICAgICAgICAgICAgICAgVXBkYXRlRGF0YSgpOwogICAgICAgICAgICB9KTsKICAgICAgICB9CgogICAgICAgIGZ1bmN0aW9uIFVwZGF0ZVRhYmxlKCkgewogICAgICAgICAgICAkKCIjanFHcmlkIikKICAgICAgICAgICAgICAgIC5qcUdyaWQoewogICAgICAgICAgICAgICAgICAgIHVybDogJ2h0dHA6Ly8xMjcuMC4wLjE6OTE5MS9kYXRhLmpzb24nLAogICAgICAgICAgICAgICAgICAgIG10eXBlOiAiR0VUIiwKICAgICAgICAgICAgICAgICAgICBhamF4U3ViZ3JpZE9wdGlvbnM6IHsKICAgICAgICAgICAgICAgICAgICAgICAgYXN5bmM6IGZhbHNlCiAgICAgICAgICAgICAgICAgICAgfSwKICAgICAgICAgICAgICAgICAgICBzdHlsZVVJOiAnQm9vdHN0cmFwJywKICAgICAgICAgICAgICAgICAgICBkYXRhdHlwZTogImpzb24iLAogICAgICAgICAgICAgICAgICAgIGNvbE1vZGVsOiBbewogICAgICAgICAgICAgICAgICAgICAgICBsYWJlbDogJyMnLAogICAgICAgICAgICAgICAgICAgICAgICBuYW1lOiAnaWQnLAogICAgICAgICAgICAgICAgICAgICAgICB3aWR0aDogNQogICAgICAgICAgICAgICAgICAgIH0sIHsKICAgICAgICAgICAgICAgICAgICAgICAgbGFiZWw6ICdCYWNrZW5kJywKICAgICAgICAgICAgICAgICAgICAgICAgbmFtZTogJ2JhY2tlbmQnLAogICAgICAgICAgICAgICAgICAgICAgICBrZXk6IHRydWUsCiAgICAgICAgICAgICAgICAgICAgICAgIHdpZHRoOiAxNQogICAgICAgICAgICAgICAgICAgIH0sIHsKICAgICAgICAgICAgICAgICAgICAgICAgbGFiZWw6ICdXZWlnaHQnLAogICAgICAgICAgICAgICAgICAgICAgICBuYW1lOiAnd2VpZ2h0JywKICAgICAgICAgICAgICAgICAgICAgICAgd2lkdGg6IDIwCiAgICAgICAgICAgICAgICAgICAgfSwgewogICAgICAgICAgICAgICAgICAgICAgICBsYWJlbDogJ1R5cGUnLAogICAgICAgICAgICAgICAgICAgICAgICBuYW1lOiAndHlwZScsCiAgICAgICAgICAgICAgICAgICAgICAgIHdpZHRoOiAyMAogICAgICAgICAgICAgICAgICAgIH1dLAogICAgICAgICAgICAgICAgICAgIHZpZXdyZWNvcmRzOiB0cnVlLAogICAgICAgICAgICAgICAgICAgIHJvd051bTogMjAsCiAgICAgICAgICAgICAgICAgICAgcGFnZXI6ICIjanFHcmlkUGFnZXIiCiAgICAgICAgICAgICAgICB9KS5qcUdyaWQoJ3NvcnRHcmlkJywgJ2lkJywgdHJ1ZSwgJ2FzYycpOwogICAgICAgIH0KCiAgICAgICAgZnVuY3Rpb24gRml4VGFibGUoKSB7CiAgICAgICAgICAgICQuZXh0ZW5kKCQuamdyaWQuYWpheE9wdGlvbnMsIHsKICAgICAgICAgICAgICAgIGFzeW5jOiBmYWxzZQogICAgICAgICAgICB9KTsKICAgICAgICAgICAgdmFyIGdyaWQgPSAkKCIjanFHcmlkIik7CiAgICAgICAgICAgIGdyaWQuc2V0R3JpZFdpZHRoKCQod2luZG93KS53aWR0aCgpIC0gNSk7CiAgICAgICAgICAgIGdyaWQuc2V0R3JpZEhlaWdodCgkKHdpbmRvdykuaGVpZ2h0KCkpOwogICAgICAgICAgICAkKHdpbmRvdykuYmluZCgncmVzaXplJywgZnVuY3Rpb24oKSB7CiAgICAgICAgICAgICAgICB2YXIganFfZ3JpZCA9ICQoIiNqcUdyaWQiKTsKICAgICAgICAgICAgICAgIGpxX2dyaWQuc2V0R3JpZFdpZHRoKCQod2luZG93KS53aWR0aCgpIC0gNSk7CiAgICAgICAgICAgICAgICBqcV9ncmlkLnNldEdyaWRIZWlnaHQoJCh3aW5kb3cpLmhlaWdodCgpKTsKICAgICAgICAgICAgfSk7CiAgICAgICAgfQoKICAgICAgICBmdW5jdGlvbiBSZW1vdmVCYWNrZW5kKCkgewogICAgICAgICAgICB2YXIgZ3JpZCA9ICQoIiNqcUdyaWQiKTsKICAgICAgICAgICAgdmFyIHJvd0tleSA9IGdyaWQuanFHcmlkKCdnZXRHcmlkUGFyYW0nLCAic2Vscm93Iik7CiAgICAgICAgICAgICQuYWpheCh7CiAgICAgICAgICAgICAgICB1cmw6ICIvcmVtb3ZlX2JhY2tlbmQiLAogICAgICAgICAgICAgICAgdHlwZTogIlBPU1QiLAogICAgICAgICAgICAgICAgZGF0YToge2JhY2tlbmQ6IHJvd0tleS5yZXBsYWNlKCJodHRwOi8vIiwgIiIpfSwKICAgICAgICAgICAgICAgIGRhdGFUeXBlOiAidGV4dCIKICAgICAgICAgICAgfSkuZG9uZShmdW5jdGlvbigpIHsKICAgICAgICAgICAgICAgIFVwZGF0ZURhdGEoKTsKICAgICAgICAgICAgfSk7CiAgICAgICAgfQoKICAgICAgICBmdW5jdGlvbiBVcGRhdGVEYXRhKCkgewogICAgICAgICAgICB2YXIgZ3JpZCA9ICQoIiNqcUdyaWQiKTsKICAgICAgICAgICAgdmFyIHJvd0tleSA9IGdyaWQuanFHcmlkKCdnZXRHcmlkUGFyYW0nLCAic2Vscm93Iik7CiAgICAgICAgICAgIGdyaWQudHJpZ2dlcigicmVsb2FkR3JpZCIpOwogICAgICAgICAgICBpZihyb3dLZXkpIHsKICAgICAgICAgICAgICAgIGdyaWQuanFHcmlkKCJyZXNldFNlbGVjdGlvbiIpOwogICAgICAgICAgICAgICAgZ3JpZC5qcUdyaWQoJ3NldFNlbGVjdGlvbicsIHJvd0tleSk7CiAgICAgICAgICAgIH0KICAgICAgICB9CgogICAgICAgIGZ1bmN0aW9uIFVwZGF0ZVN0YXRzKCkgewogICAgICAgICAgICAkLmFqYXgoewogICAgICAgICAgICAgICAgdXJsOiAiL3N0YXRzLmpzb24iLAogICAgICAgICAgICAgICAgdHlwZTogIkdFVCIsCiAgICAgICAgICAgICAgICBkYXRhVHlwZTogInRleHQiCiAgICAgICAgICAgIH0pLmRvbmUoZnVuY3Rpb24oZGF0YSkgewogICAgICAgICAgICAgICAgJCgnI3N0YXRzJykudGV4dChkYXRhKTsKICAgICAgICAgICAgfSk7CgogICAgICAgIH0KCiAgICAgICAgZnVuY3Rpb24gT25Mb2FkKCkgewogICAgICAgICAgICBVcGRhdGVUYWJsZSgpOwogICAgICAgICAgICBGaXhUYWJsZSgpOwogICAgICAgICAgICBzZXRJbnRlcnZhbChVcGRhdGVEYXRhLCAxNTAwMCk7CiAgICAgICAgICAgIHNldEludGVydmFsKFVwZGF0ZVN0YXRzLCA1MDAwKTsKICAgICAgICB9CiAgICA8L3NjcmlwdD4KPC9oZWFkPgoKPGJvZHkgb25sb2FkPSJPbkxvYWQoKSI+CjxkaXYgY2xhc3M9Im5hdmJhciBuYXZiYXItaW52ZXJzZSBuYXZiYXItZml4ZWQtdG9wIj4KICAgIDxkaXYgY2xhc3M9ImNvbnRhaW5lciI+CiAgICAgICAgPGRpdiBjbGFzcz0ibmF2YmFyLWhlYWRlciI+CiAgICAgICAgICAgIDxidXR0b24gdHlwZT0iYnV0dG9uIiBjbGFzcz0ibmF2YmFyLXRvZ2dsZSIgZGF0YS10b2dnbGU9ImNvbGxhcHNlIiBkYXRhLXRhcmdldD0iLm5hdmJhci1jb2xsYXBzZSI+CiAgICAgICAgICAgICAgICA8c3BhbiBjbGFzcz0iaWNvbi1iYXIiPjwvc3Bhbj48c3BhbiBjbGFzcz0iaWNvbi1iYXIiPjwvc3Bhbj48c3BhbiBjbGFzcz0iaWNvbi1iYXIiPjwvc3Bhbj4KICAgICAgICAgICAgPC9idXR0b24+CiAgICAgICAgICAgIDxhIGNsYXNzPSJuYXZiYXItYnJhbmQiIGhyZWY9IiMiPkxCIFdFQjwvYT4KICAgICAgICA8L2Rpdj4KICAgICAgICA8ZGl2IGNsYXNzPSJuYXZiYXItY29sbGFwc2UgY29sbGFwc2UiPgogICAgICAgICAgICA8dWwgY2xhc3M9Im5hdiBuYXZiYXItbmF2Ij4KICAgICAgICAgICAgICAgIDxsaSBjbGFzcz0iZHJvcGRvd24iPgogICAgICAgICAgICAgICAgICAgIDxhIGhyZWY9IiMiIGNsYXNzPSJkcm9wZG93bi10b2dnbGUiIGRhdGEtdG9nZ2xlPSJkcm9wZG93biI+QmFja2VuZHMgPGIgY2xhc3M9ImNhcmV0Ij48L2I+PC9hPgogICAgICAgICAgICAgICAgICAgIDx1bCBjbGFzcz0iZHJvcGRvd24tbWVudSI+CiAgICAgICAgICAgICAgICAgICAgICAgIDxsaT4KICAgICAgICAgICAgICAgICAgICAgICAgICAgIDxhIGRhdGEtdG9nZ2xlPSJtb2RhbCIgZGF0YS10YXJnZXQ9IiNhZGRCYWNrZW5kIj5BZGQgQmFja2VuZDwvYT4KICAgICAgICAgICAgICAgICAgICAgICAgPC9saT4KICAgICAgICAgICAgICAgICAgICAgICAgPGxpIG9uY2xpY2s9IlJlbW92ZUJhY2tlbmQoKSI+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICA8YSBocmVmPSIjIj5EZWxldGUgQmFja2VuZDwvYT4KICAgICAgICAgICAgICAgICAgICAgICAgPC9saT4KICAgICAgICAgICAgICAgICAgICA8L3VsPgogICAgICAgICAgICAgICAgPC9saT4KICAgICAgICAgICAgICAgIDxsaT4KICAgICAgICAgICAgICAgICAgICA8YSBkYXRhLXRvZ2dsZT0ibW9kYWwiIGRhdGEtdGFyZ2V0PSIjc2hvd1N0YXRpc3RpY3MiPlN0YXRpc3RpY3M8L2E+CiAgICAgICAgICAgICAgICA8L2xpPgogICAgICAgICAgICAgICAgPGxpPgogICAgICAgICAgICAgICAgICAgIDxhIGRhdGEtdG9nZ2xlPSJtb2RhbCIgZGF0YS10YXJnZXQ9IiNhYm91dFdpbmRvdyI+QWJvdXQ8L2E+CiAgICAgICAgICAgICAgICA8L2xpPgogICAgICAgICAgICA8L3VsPgogICAgICAgIDwvZGl2PgogICAgICAgIDwhLS0vLm5hdmJhci1jb2xsYXBzZSAtLT4KICAgIDwvZGl2Pgo8L2Rpdj4KPC9QPgo8dGFibGUgaWQ9ImpxR3JpZCI+PC90YWJsZT4KCjwhLS0gTW9kYWwgLS0+CjxkaXYgY2xhc3M9Im1vZGFsIGZhZGUiIGlkPSJhZGRCYWNrZW5kIiByb2xlPSJkaWFsb2ciPgogICAgPGRpdiBjbGFzcz0ibW9kYWwtZGlhbG9nIj4KICAgICAgICA8IS0tIE1vZGFsIGNvbnRlbnQtLT4KICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1jb250ZW50Ij4KICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtaGVhZGVyIj4KICAgICAgICAgICAgICAgIDxidXR0b24gdHlwZT0iYnV0dG9uIiBjbGFzcz0iY2xvc2UiIGRhdGEtZGlzbWlzcz0ibW9kYWwiPiZ0aW1lczs8L2J1dHRvbj4KICAgICAgICAgICAgICAgIDxoNCBjbGFzcz0ibW9kYWwtdGl0bGUiPkJhY2tlbmQ8L2g0PgogICAgICAgICAgICA8L2Rpdj4KICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtYm9keSI+CiAgICAgICAgICAgICAgICA8ZGl2IGNsYXNzPSJmb3JtLWdyb3VwIj4KICAgICAgICAgICAgICAgICAgICA8bGFiZWwgY2xhc3M9ImNvbnRyb2wtbGFiZWwiPlVybDwvbGFiZWw+CiAgICAgICAgICAgICAgICAgICAgPGRpdiBjbGFzcz0iY29udHJvbHMiPgogICAgICAgICAgICAgICAgICAgICAgICA8aW5wdXQgdHlwZT0idGV4dCIgaWQ9InVybF9pZCIgY2xhc3M9ImZvcm0tY29udHJvbCIgdmFsdWU9Imh0dHA6Ly8xMjcuMC4wLjE6ODA4MSI+CiAgICAgICAgICAgICAgICAgICAgPC9kaXY+CgogICAgICAgICAgICAgICAgICAgIDxsYWJlbCBjbGFzcz0iY29udHJvbC1sYWJlbCI+V2VpZ2h0PC9sYWJlbD4KICAgICAgICAgICAgICAgICAgICA8ZGl2IGNsYXNzPSJjb250cm9scyI+CiAgICAgICAgICAgICAgICAgICAgICAgIDxpbnB1dCB0eXBlPSJ0ZXh0IiBpZD0id2VpZ2h0X2lkIiBjbGFzcz0iZm9ybS1jb250cm9sIiB2YWx1ZT0iMSI+CiAgICAgICAgICAgICAgICAgICAgPC9kaXY+CgogICAgICAgICAgICAgICAgICAgIDxsYWJlbCBjbGFzcz0iY29udHJvbC1sYWJlbCI+VHlwZTwvbGFiZWw+CiAgICAgICAgICAgICAgICAgICAgPHNlbGVjdCBjbGFzcz0iZm9ybS1jb250cm9sIiBpZD0idHlwZV9pZCI+CiAgICAgICAgICAgICAgICAgICAgICAgIDxvcHRpb24+c3RhbmRhcmQ8L29wdGlvbj4KICAgICAgICAgICAgICAgICAgICAgICAgPG9wdGlvbj5taXJyb3I8L29wdGlvbj4KICAgICAgICAgICAgICAgICAgICAgICAgPG9wdGlvbj5kb3duPC9vcHRpb24+CiAgICAgICAgICAgICAgICAgICAgICAgIDxvcHRpb24+YmFja3VwPC9vcHRpb24+CiAgICAgICAgICAgICAgICAgICAgPC9zZWxlY3Q+CgogICAgICAgICAgICAgICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWZvb3RlciI+CiAgICAgICAgICAgICAgICAgICAgICAgIDxhIGNsYXNzPSJidG4gYnRuLXByaW1hcnkiIG9uY2xpY2s9IkFkZEJhY2tlbmQoKSIgZGF0YS1kaXNtaXNzPSJtb2RhbCI+QWRkIGJhY2tlbmQ8L2E+CiAgICAgICAgICAgICAgICAgICAgPC9kaXY+CiAgICAgICAgICAgICAgICA8L2Rpdj4KICAgICAgICAgICAgPC9kaXY+CiAgICAgICAgPC9kaXY+CiAgICA8L2Rpdj4KPC9kaXY+Cgo8IS0tIE1vZGFsIC0tPgo8ZGl2IGNsYXNzPSJtb2RhbCBmYWRlIiBpZD0iYWJvdXRXaW5kb3ciIHJvbGU9ImRpYWxvZyI+CiAgICA8ZGl2IGNsYXNzPSJtb2RhbC1kaWFsb2ciPgogICAgICAgIDwhLS0gTW9kYWwgY29udGVudC0tPgogICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWNvbnRlbnQiPgogICAgICAgICAgICA8ZGl2IGNsYXNzPSJtb2RhbC1oZWFkZXIiPgogICAgICAgICAgICAgICAgPGJ1dHRvbiB0eXBlPSJidXR0b24iIGNsYXNzPSJjbG9zZSIgZGF0YS1kaXNtaXNzPSJtb2RhbCI+JnRpbWVzOzwvYnV0dG9uPgogICAgICAgICAgICAgICAgPGg0IGNsYXNzPSJtb2RhbC10aXRsZSI+QWJvdXQ8L2g0PgogICAgICAgICAgICA8L2Rpdj4KICAgICAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtYm9keSI+CiAgICAgICAgICAgICAgICA8cD4KICAgICAgICAgICAgICAgICAgICA8c3Ryb25nPk5BTUU6PC9zdHJvbmc+PC9icj4KICAgICAgICAgICAgICAgICAgICAmbmJzcDsmbmJzcDsmbmJzcDsmbmJzcDtscGctbG9hZC1iYWxhbmNlciAtIFNpbXBsZSBsb2FkIGJhbGFuY2VyIHdpdGggSHR0cCBBUEkuCiAgICAgICAgICAgICAgICA8L3A+CiAgICAgICAgICAgICAgICA8cD4KICAgICAgICAgICAgICAgICAgICA8c3Ryb25nPkFVVEhPUihTKTo8L3N0cm9uZz48L2JyPgogICAgICAgICAgICAgICAgICAgICZuYnNwOyZuYnNwOyZuYnNwOyZuYnNwO0dvVExpdU0gSW5TUGlSaVQgPGdvdGxpdW1AZ21haWwuY29tPgogICAgICAgICAgICAgICAgPC9wPgogICAgICAgICAgICA8L2Rpdj4KICAgICAgICA8L2Rpdj4KICAgIDwvZGl2Pgo8L2Rpdj4KCjwhLS0gTW9kYWwgLS0+CjxkaXYgY2xhc3M9Im1vZGFsIGZhZGUiIGlkPSJzaG93U3RhdGlzdGljcyIgcm9sZT0iZGlhbG9nIj4KICAgIDxkaXYgY2xhc3M9Im1vZGFsLWRpYWxvZyI+CiAgICAgICAgPCEtLSBNb2RhbCBjb250ZW50LS0+CiAgICAgICAgPGRpdiBjbGFzcz0ibW9kYWwtY29udGVudCI+CiAgICAgICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWhlYWRlciI+CiAgICAgICAgICAgICAgICA8YnV0dG9uIHR5cGU9ImJ1dHRvbiIgY2xhc3M9ImNsb3NlIiBkYXRhLWRpc21pc3M9Im1vZGFsIj4mdGltZXM7PC9idXR0b24+CiAgICAgICAgICAgICAgICA8aDQgY2xhc3M9Im1vZGFsLXRpdGxlIj5TdGF0aXN0aWNzPC9oND4KICAgICAgICAgICAgPC9kaXY+CiAgICAgICAgICAgIDxkaXYgY2xhc3M9Im1vZGFsLWJvZHkiPgogICAgICAgICAgICAgICAgPHByZSBpZD0ic3RhdHMiPjwvcHJlPgogICAgICAgICAgICA8L2Rpdj4KICAgICAgICA8L2Rpdj4KICAgIDwvZGl2Pgo8L2Rpdj4KPC9ib2R5Pgo8L2h0bWw+Cg=="

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
