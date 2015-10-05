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
	"encoding/json"

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
	"git.lpgenerator.ru/sys/lpg-load-balancer/common"
	"git.lpgenerator.ru/sys/lpg-load-balancer/helpers"
	service "github.com/ayufan/golang-kardianos-service"
	"git.lpgenerator.ru/sys/lpg-load-balancer/helpers/service"
	"git.lpgenerator.ru/sys/lpg-load-balancer/commands/mirroring"
	"git.lpgenerator.ru/sys/lpg-load-balancer/commands/statistics"
	"git.lpgenerator.ru/sys/lpg-load-balancer/commands/monitoring"
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


func HandleIndex(w http.ResponseWriter, r *http.Request) {
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
		u, err := url.Parse("http://" + s[1])
		if err != nil {
			setStatus(w, "ERROR")
		} else {
			srv_type := "standard"
			for _, server := range LB.config.Servers {
				if server.Url == fmt.Sprintf("http://%s", s[1]) {
					srv_type = server.Type
				}
			}
			err := LB.lb.RemoveServer(u)
			if (srv_type == "standard" || srv_type == "") && err != nil {
				setStatus(w, "ERROR")
				log.Errorf("failed to remove %s, err: %v", s[1], err)
			} else {
				removeServerFromConfig(s[1])
				setStatus(w, "OK")
				log.Infof("%s was removed", s[1])
			}
		}
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

func HandleStats(w http.ResponseWriter, r *http.Request) {
	data, err := json.MarshalIndent(LB.stats.Data(), "", "  ")
	setHttpHeaders(w)
	if err == nil {
		io.WriteString(w, string(data))
	} else {
		setStatus(w, "ERROR")
	}
}

func HandleStatus(w http.ResponseWriter, r *http.Request) {
	updateSystemStatus()

	stats, err := json.MarshalIndent(sysStatus, "", "  ")
	if err == nil {
		io.WriteString(w, string(stats))
	} else {
		setStatus(w, "ERROR")
	}
}

func (mr *RunCommand) Run() {
	go func() {
		http.HandleFunc(
			"/", helpers.BasicAuth(
			helpers.LogRequests(HandleIndex),
			mr.config.LbApiLogin, mr.config.LbApiPassword))
		http.HandleFunc(
			"/stats", helpers.BasicAuth(
			helpers.LogRequests(HandleStats),
			mr.config.LbApiLogin, mr.config.LbApiPassword))
		http.HandleFunc(
			"/status", helpers.BasicAuth(
			helpers.LogRequests(HandleStatus),
			mr.config.LbApiLogin, mr.config.LbApiPassword))
		log.Println("LB API listen at", mr.config.ApiAddress)
		http.ListenAndServe(mr.config.ApiAddress, nil)
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

	// Trace Middleware
	trc_log, _ := os.OpenFile(
		mr.config.LbTaceFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	trc_mw, _ := trace.New(fwd, trc_log)
	trc := getNextHandler(trc_mw, fwd, mr.config.LbEnableTace, "Tracing")

	mrr_mw, _ := mirror.New(trc)
	mrr := getNextHandler(mrr_mw, trc, mr.config.LbStats, "Mirroring")

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

	//todo: memetrics mw

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
