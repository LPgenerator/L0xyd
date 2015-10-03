package monitoring

import (
	"fmt"
	"time"
	"os/exec"
	"net/url"
	"net/http"
	"io/ioutil"
	"github.com/mailgun/oxy/utils"
	"github.com/mailgun/oxy/roundrobin"
	"git.lpgenerator.ru/sys/lpg-load-balancer/common"
)

type Backend struct {
	ResponseCounts      map[int]int
	State               bool
}

type Monitoring struct {
	next        http.Handler
	backends    map[string]Backend
	lb          *roundrobin.RoundRobin
	config      *common.Config
}

func New(next http.Handler, config *common.Config) (*Monitoring, error) {
	strm := &Monitoring{
		backends: make(map[string]Backend),
		next:     next,
		config:   config,
	}
	return strm, nil
}

func (m *Monitoring) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("X-A", "LP-LB")
	pw := &utils.ProxyWriter{W: w}
	m.next.ServeHTTP(pw, r)

	backend := r.URL.String()
	if len(m.backends[backend].ResponseCounts) == 0 {
		m.backends[backend] = Backend{
			State:          true,
			ResponseCounts: make(map[int]int),
		}
	}
	if m.backends[backend].ResponseCounts[pw.Code] == 0 {
		m.backends[backend].ResponseCounts[pw.Code] = 0
	}

	m.backends[backend].ResponseCounts[pw.Code]++
}

func (m *Monitoring) Start(lb *roundrobin.RoundRobin) {
	if m.config.LbMonitorBrokenBackends == true {
		m.lb = lb
		m.doStart()
	}
}

func (m *Monitoring) doStart() {
	check_period := time.Duration(m.config.LbMonitorCheckPeriod) * time.Second
	max_fails := m.config.LbMonitorMaxFails
	fmt.Sprintf("check_period: %d", check_period)

	for {
		for backend, _ := range m.backends {
			if m.backends[backend].State == false { continue }
			if m.backends[backend].ResponseCounts[502] > max_fails {
				m.removeBackend(backend)
				m.callNotificationUrl(backend)
				m.callNotificationScript(backend)
				delete(m.backends[backend].ResponseCounts, 502)
			}
		}
		time.Sleep(check_period)
		m.cleanupFailTimeout()
	}
}

func (m *Monitoring) cleanupFailTimeout() {
	// TODO: fail_timeout
	// удалять после определенного времени данные по 502
	// потому как есть вероятность что 502 может быть переодически
	//delete(m.backends[backend].ResponseCounts, 502)
}

func (m *Monitoring) removeBackend(backend string) {
	if m.config.LbMonitorRemoveBrokenBackends == false { return }
	u, _ := url.Parse(backend)
	if err := m.lb.RemoveServer(u); err == nil {
		time.Sleep(3 * time.Second)
		var bak = m.backends[backend]
		bak.State = false
		m.backends[backend] = bak
		//delete(m.backends[backend].ResponseCounts, 502)
		fmt.Println(m.backends[backend])
	}
}

func (m *Monitoring) callNotificationScript(backend string) {
	if m.config.LbMonitorBashScript == "" { return }
	exec.Command("bash", "-c", m.config.LbMonitorBashScript, backend).Output()
}

func (m *Monitoring) callNotificationUrl(backend string) {
	if m.config.LbMonitorWebUrl == "" { return }
	uri := fmt.Sprintf("%s/?b=%s", m.config.LbMonitorWebUrl, backend)
	r, err := http.NewRequest("GET", uri, nil)
	if err == nil {
		re, err := http.DefaultClient.Do(r)
		if err == nil {
			if re.Body != nil {
				defer re.Body.Close()
			}
			ioutil.ReadAll(re.Body)
		}
	}
}
