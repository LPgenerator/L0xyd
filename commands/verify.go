package commands

import (
	"github.com/codegangsta/cli"
	log "github.com/Sirupsen/logrus"
	"github.com/LPgenerator/lpg-load-balancer/common"
)

type VerifyCommand struct {
	configOptions
}

func (c *VerifyCommand) Execute(context *cli.Context) {
	err := c.loadConfig()
	if err != nil {
		log.Fatalln(err)
		return
	}
	log.Println("OK")
}

func init() {
	common.RegisterCommand2("verify", "Verify configuration", &VerifyCommand{})
}
