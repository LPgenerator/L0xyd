package commands

import (
	"os"
	"io"
	"fmt"
	"strconv"
	"strings"
	"net/url"
	"net/http"
	"encoding/json"

	"github.com/codegangsta/cli"

	"github.com/mailgun/oxy/utils"
	"github.com/mailgun/oxy/stream"
	"github.com/mailgun/oxy/forward"
	"github.com/mailgun/oxy/roundrobin"

	log "github.com/Sirupsen/logrus"
	"git.lpgenerator.ru/sys/lpg-load-balancer/common"
	"git.lpgenerator.ru/sys/lpg-load-balancer/helpers"
	service "github.com/ayufan/golang-kardianos-service"
	"git.lpgenerator.ru/sys/lpg-load-balancer/helpers/service"
)

type Server struct {
	Url           string
	Weight        int
}


type RunCommand struct {
	configOptions

	ListenAddr string `short:"l" long:"listen" description:"Listen address:port"`
	ServiceName      string `short:"n" long:"service" description:"Use different names for different services"`
	WorkingDirectory string `short:"d" long:"working-directory" description:"Specify custom working directory"`
	Syslog           bool   `long:"syslog" description:"Log to syslog"`
}

var LB struct {
	lb *roundrobin.RoundRobin
	config *common.Config
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
		}
	}
}

func addServerToConfig(s string, v int) {
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
	}
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Server", "lpgenerator.ru")

	if r.Method == "PUT" {
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
	v, err := strconv.Atoi(r.Form.Get("weight"))
	if err != nil { v = 0 }
	if s != "" {
		u, err := url.Parse("http://" + s)
		if err != nil {
			setStatus(w, "ERROR")
		} else {
			if err := LB.lb.UpsertServer(u, roundrobin.Weight(v)); err != nil {
				setStatus(w, "ERROR")
				log.Errorf("failed to add %s, err: %s", s, err)
			} else {
				removeServerFromConfig(s)
				addServerToConfig(s, v)
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
			if err := LB.lb.RemoveServer(u); err != nil {
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
	servers := []Server{}
	for _, url := range LB.lb.Servers() {
		w, _ := LB.lb.ServerWeight(url)
		servers = append(servers,  Server{Url: url.String(), Weight: w})
	}

	data, err := json.Marshal(servers)
	if err == nil {
		io.WriteString(w, string(data))
	} else {
		setStatus(w, "ERROR")
	}
}

func (mr *RunCommand) Run() {
	go func() {
		http.HandleFunc(
			"/", helpers.BasicAuth(helpers.LogRequests(HandleIndex)))
		log.Println("LB API listen at", mr.config.ApiAddress)
		http.ListenAndServe(mr.config.ApiAddress, nil)
	}()

	oxyLogger := &helpers.OxyLogger{}
	fwd_logger := forward.Logger(oxyLogger)
	stm_logger := stream.Logger(oxyLogger)

	if mr.config.LbLogFile != "" {
		f, err := os.OpenFile(
			mr.config.LbLogFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		} else {
			fileLogger := utils.NewFileLogger(f, utils.INFO)
			fwd_logger = forward.Logger(fileLogger)
			stm_logger = stream.Logger(fileLogger)
			defer f.Close()
		}
	}

	fwd, _ := forward.New(fwd_logger)
	lb, _ := roundrobin.New(fwd)

	LB.lb = lb
	LB.config = mr.config

	stream, _ := stream.New(
		lb, stm_logger, stream.Retry(
			`IsNetworkError() && RequestMethod() == "GET" && Attempts() < 2`))

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
		if err == nil {
			lb.UpsertServer(u, roundrobin.Weight(server.Weight))
			log.Printf(
				"LB: %s (Url=%s, Weight=%d) was added.",
				serverName, server.Url, server.Weight)
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
	common.RegisterCommand2("run",  "Run Load Balancer", &RunCommand{
		ServiceName: defaultServiceName,
	})
}
