package main

import (
	"fmt"
	"os"
	"path"
	//"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"

	_ "git.lpgenerator.ru/sys/lpg-load-balancer/commands"
	"git.lpgenerator.ru/sys/lpg-load-balancer/common"
	"git.lpgenerator.ru/sys/lpg-load-balancer/helpers"
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
	// < Go 1.5
	// runtime.GOMAXPROCS(runtime.NumCPU())

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
