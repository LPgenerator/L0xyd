package commands

import (
	"github.com/codegangsta/cli"

	log "github.com/Sirupsen/logrus"
	"github.com/gotlium/lpg-load-balancer/common"
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

	// save config file
	err = c.saveConfig()
	if err != nil {
		log.Fatalln("Failed to update", c.ConfigFile, err)
	}

	log.Println("Updated")
}

func init() {
	common.RegisterCommand2("verify",  "verify all", &VerifyCommand{})
}
