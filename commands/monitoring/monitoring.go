package monitoring

import (
	"fmt"
	"sync"
	"time"
	"os/exec"
	"net/url"
	"net/http"
	"io/ioutil"
	"github.com/mailgun/timetools"
	"github.com/mailgun/oxy/utils"
	//log "github.com/Sirupsen/logrus"
	"github.com/mailgun/oxy/roundrobin"
	"github.com/LPgenerator/lpg-load-balancer/common"
)

type Backend struct {
	ResponseCounts  map[int]int
	State           bool
}

type Monitoring struct {
	m               *sync.RWMutex
	next            http.Handler
	backends        map[string]Backend
	lb              *roundrobin.RoundRobin
	config          *common.Config
	clock           timetools.TimeProvider
	interval        time.Time
}

func New(next http.Handler, config *common.Config) (*Monitoring, error) {
	strm := &Monitoring{
		backends:   make(map[string]Backend),
		m:          &sync.RWMutex{},
		next:       next,
		config:     config,
		clock:      &timetools.RealTime{},
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
	fail_timeout := time.Duration(m.config.LbMonitorFailTimeout) * time.Second
	max_fails := m.config.LbMonitorMaxFails

	for {
		for backend, _ := range m.backends {
			if m.backends[backend].State == false { continue }
			if m.backends[backend].ResponseCounts[502] > max_fails {
				m.removeBackend(backend)
				m.addBackupServer()
				m.callNotificationUrl(backend)
				m.callNotificationScript(backend)
				m.safeRemove502(backend)
			}
		}
		time.Sleep(check_period)
		m.cleanupFailTimeout(fail_timeout)
	}
}

func (m *Monitoring) safeRemove502(backend string) {
	m.m.Lock()
	defer m.m.Unlock()
	delete(m.backends[backend].ResponseCounts, 502)
}

func (m *Monitoring) cleanupFailTimeout(fail_timeout time.Duration) {
	if !m.clock.UtcNow().After(m.interval) { return }
	m.interval = m.clock.UtcNow().Add(fail_timeout)

	for backend, _ := range m.backends {
		if m.backends[backend].State == false { continue }
		if m.backends[backend].ResponseCounts[502] > 0 {
			m.safeRemove502(backend)
		}
	}
}

func (m *Monitoring) removeBackend(backend string) {
	if m.config.LbMonitorRemoveBrokenBackends == false { return }
	u, _ := url.Parse(backend)
	if err := m.lb.RemoveServer(u); err == nil {
		time.Sleep(3 * time.Second)
		var bak = m.backends[backend]
		bak.State = false
		m.backends[backend] = bak
		// todo: mark as removed on basic config
	}
}

func (m *Monitoring) addBackupServer() {
	//todo: do it
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
