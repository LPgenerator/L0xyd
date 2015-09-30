package commands

import (
	"os"
	"io"
	"fmt"
	"strings"
	"net/url"
	"net/http"
	"encoding/json"

	"github.com/codegangsta/cli"

	"github.com/mailgun/oxy/stream"
	"github.com/mailgun/oxy/forward"
	"github.com/mailgun/oxy/roundrobin"

	log "github.com/Sirupsen/logrus"
	"github.com/gotlium/lpg-load-balancer/common"
    "github.com/gotlium/lpg-load-balancer/helpers"
)

type LBCommand struct {
	configOptions

	ListenAddr string `short:"l" long:"listen" description:"Listen address:port"`
	// ApiAddr string `short:"a" long:"api-listen" description:"Api listen address:port"`
}

var LB struct {
	lb *roundrobin.RoundRobin
}

func setStatus(w http.ResponseWriter, status string) {
	w.Header().Set("Content-Type", "application/json")
	if status == "ERROR" {
		http.Error(
			w, fmt.Sprintf(`{"status": "%s"}`, status),
			http.StatusInternalServerError)
	} else {
		// w.WriteHeader(http.StatusCreated)
		io.WriteString(w, fmt.Sprintf(`{"status": "%s"}`, status))
	}
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE")
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
	if s != "" {
		u, err := url.Parse(s)
		if err != nil {
			setStatus(w, "ERROR")
		} else {
			if err := LB.lb.UpsertServer(u); err != nil {
				setStatus(w, "ERROR")
				log.Errorf("failed to add %s, err: %s", s, err)
			} else {
				setStatus(w, "OK")
				log.Infof("%s was added", s)
			}
		}
	} else {
		setStatus(w, "ERROR")
	}
}

func HandleDel(w http.ResponseWriter, r *http.Request) {
	s := strings.Split(r.URL.Path, "/")
	if len(s) == 2 {
		u, err := url.Parse(s[1])
		if err != nil {
			setStatus(w, "ERROR")
		} else {
			if err := LB.lb.RemoveServer(u); err != nil {
				setStatus(w, "ERROR")
				log.Errorf("failed to remove %s, err: %v", s, err)
			} else {
				setStatus(w, "OK")
				log.Infof("%s was removed", s[1])
			}
		}
	} else {
		setStatus(w, "ERROR")
	}
}

func HandleList(w http.ResponseWriter, r *http.Request) {
	servers := LB.lb.Servers()
	data, err := json.Marshal(servers)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, string(data))
	} else {
		setStatus(w, "ERROR")
	}
}


func (c *LBCommand) Execute(context *cli.Context) {

	go func() {
		http.HandleFunc(
			"/", helpers.BasicAuth(helpers.LogRequests(HandleIndex)))
		// http.HandleFunc("/add/", helpers.LogRequests(HandleAdd))
		// http.HandleFunc("/del/", helpers.LogRequests(HandleDel))
		// http.HandleFunc("/list/", helpers.LogRequests(HandleList))

		http.ListenAndServe(":8182", nil)
	}()

	oxyLogger := &helpers.OxyLogger{}

	// l := utils.NewFileLogger(os.Stdout, utils.INFO)
	fwd, _ := forward.New(forward.Logger(oxyLogger))
	lb, _ := roundrobin.New(fwd)
	LB.lb = lb

	stream, _ := stream.New(
		lb, stream.Logger(oxyLogger), stream.Retry(
			`IsNetworkError() && RequestMethod() == "GET" && Attempts() < 2`))

	listen := ":8080"
	if c.ListenAddr != "" {
		listen = c.ListenAddr
	}

	s := &http.Server{
		Addr:           listen,
		Handler:        stream,
	}

	log.Println("Load Balancer listen at", listen)

	if err := s.ListenAndServe(); err != nil {
		log.Errorf("Server %s exited with error: %s", s.Addr, err)
		os.Exit(255)
	}
}


func init() {
	common.RegisterCommand2("run",  "Run Load Balancer", &LBCommand{})
}
