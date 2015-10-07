package commands

import (
	"os"
	"io"
	"fmt"
	"strings"
	"net/http"
	"net/http/httputil"

	"github.com/codegangsta/cli"
	"github.com/codegangsta/negroni"

	log "github.com/Sirupsen/logrus"
	"github.com/LPgenerator/L0xyd/common"
	"github.com/LPgenerator/L0xyd/helpers"
)

type SHSCommand struct {
	configOptions

	ListenAddr string `short:"l" long:"listen" description:"Listen address:port"`
	StdOut     bool `short:"s" long:"stdout" description:"Log to StdOut"`
	HttpHeader bool `short:"p" long:"headers" description:"Request headers"`
}

var SHS struct {
	c          *SHSCommand
}


func HandleMain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-control", `no-cache="set-cookie"`)
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Vary", `Cookie, Accept-Encoding`)
	w.Header().Set("Server", "nginx")
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("X-Listen", r.Host)

	io.WriteString(w, fmt.Sprintf("[%s] Hello, World!", r.Host))

	if r.Method == "POST" {
		r.ParseForm()
		log.Println(r.Form)
	}
	if SHS.c.HttpHeader {
		data, _ := httputil.DumpRequest(r, true)
		log.Print(strings.TrimSpace(string(data)))
	}
}


func (c *SHSCommand) Execute(context *cli.Context) {
	SHS.c = c

	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleMain)

	n := negroni.New()
	n.UseHandler(mux)
	if c.StdOut {
		n.Use(helpers.LogMiddleware())
	}

	listen := "127.0.0.1:8282"
	if c.ListenAddr != "" {
		listen = c.ListenAddr
	}

	log.Println("HTTP Server listen at", listen)
	if err := http.ListenAndServe(listen, n); err != nil {
		log.Errorf("Server exited with error: %s", err)
		os.Exit(255)
	}
}


func init() {
	common.RegisterCommand2("http", "Run simple HTTP server", &SHSCommand{})
}
