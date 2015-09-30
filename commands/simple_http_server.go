package commands

import (
	"os"
	"io"
	"fmt"
	"net/http"

	"github.com/codegangsta/cli"

	log "github.com/Sirupsen/logrus"
	"github.com/gotlium/lpg-load-balancer/common"
    "github.com/gotlium/lpg-load-balancer/helpers"
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
	io.WriteString(w, fmt.Sprintf("[%s] Hello, World!", SHS.listen))
}


func (c *SHSCommand) Execute(context *cli.Context) {
	http.HandleFunc("/", helpers.LogRequests(HandleMain))

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
