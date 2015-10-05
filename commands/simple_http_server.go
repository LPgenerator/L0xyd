package commands

import (
	"os"
	"io"
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/codegangsta/cli"

	log "github.com/Sirupsen/logrus"
	"github.com/LPgenerator/lpg-load-balancer/common"
	"github.com/LPgenerator/lpg-load-balancer/helpers"
)

type SHSCommand struct {
	configOptions

	ListenAddr string `short:"l" long:"listen" description:"Listen address:port"`
	StdOut     bool `short:"s" long:"stdout" description:"Log to StdOut"`
}

var SHS struct {
	listen string
}


func captureHeaders(in http.Header, headers []string) http.Header {
	if len(headers) == 0 || in == nil {
		return nil
	}
	out := make(http.Header, len(headers))
	for _, h := range headers {
		vals, ok := in[h]
		if !ok || len(out[h]) != 0 {
			continue
		}
		for i := range vals {
			out.Add(h, vals[i])
		}
	}
	return out
}

func HandleMain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-control", `no-cache="set-cookie"`)
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Vary", `Cookie, Accept-Encoding`)
	w.Header().Set("Server", "nginx")
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("X-Listen", SHS.listen)
	io.WriteString(w, fmt.Sprintf("[%s] Hello, World!", SHS.listen))

	if r.Method == "POST" {
		r.ParseForm()
		log.Println(r.Form)
	}
	data, _ := httputil.DumpRequest(r, true)
	log.Print(string(data))
}


func (c *SHSCommand) Execute(context *cli.Context) {
	if c.StdOut {
		http.HandleFunc("/", helpers.LogRequests(HandleMain))
	} else {
		http.HandleFunc("/", HandleMain)
	}

	listen := ":8081"
	if c.ListenAddr != "" {
		listen = c.ListenAddr
	}
    SHS.listen = listen

	log.Println("HTTP Server listen at", listen)

	if err := http.ListenAndServe(listen, nil); err != nil {
		log.Errorf("Server exited with error: %s", err)
		os.Exit(255)
	}
}


func init() {
	common.RegisterCommand2("http", "Run simple HTTP server", &SHSCommand{})
}
