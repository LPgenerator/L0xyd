package commands

import (
	"os"
	"io"
	"fmt"
	"net/http"

	"github.com/codegangsta/cli"

	log "github.com/Sirupsen/logrus"
	"git.lpgenerator.ru/sys/lpg-load-balancer/common"
	//"git.lpgenerator.ru/sys/lpg-load-balancer/helpers"
	//"github.com/mailgun/oxy/trace"
)

type SHSCommand struct {
	configOptions

	ListenAddr string `short:"l" long:"listen" description:"Listen address:port"`
}

var SHS struct {
	listen string
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
}


func (c *SHSCommand) Execute(context *cli.Context) {
	// http.HandleFunc("/", helpers.LogRequests(HandleMain))
	http.HandleFunc("/", HandleMain)

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
	common.RegisterCommand2("http",  "Run simple HTTP server", &SHSCommand{})
}
