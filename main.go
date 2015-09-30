package main

import (
	"os"
	"fmt"
	"path"
	"runtime"

	"github.com/codegangsta/cli"
	log "github.com/Sirupsen/logrus"

	"github.com/gotlium/lpg-load-balancer/common"
	"github.com/gotlium/lpg-load-balancer/helpers"
	_ "github.com/gotlium/lpg-load-balancer/commands"
)

var NAME = "lpg-load-balancer"
var VERSION = "dev"
var REVISION = "HEAD"


func init() {
	common.NAME = NAME
	common.VERSION = VERSION
	common.REVISION = REVISION
}


func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	app := cli.NewApp()
	app.Name = path.Base(os.Args[0])
	app.Usage = "lpg-load-balancer"
	app.Version = fmt.Sprintf("%s (%s)", common.VERSION, common.REVISION)
	app.Author = "GoTLiuM InSPiRiT"
	app.Email = "gotlium@gmail.com"
	helpers.SetupLogLevelOptions(app)
	app.Commands = common.GetCommands()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
