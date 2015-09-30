package helpers

import (
	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"log/syslog"
	"os"
)

func SetupLogLevelOptions(app *cli.App) {
	newFlags := []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "debug mode",
			EnvVar: "DEBUG",
		},
		cli.StringFlag{
			Name:  "log-level, l",
			Value: "error",
			Usage: "Log level (options: debug, info, warn, error, fatal, panic)",
		},
		cli.StringFlag{
			Name:  "syslog",
			Value: "localhost:514",
			Usage: "Send messages to syslog",
		},
	}
	app.Flags = append(app.Flags, newFlags...)

	appBefore := app.Before
	// logs
	app.Before = func(c *cli.Context) error {
		if c.IsSet("syslog") && c.String("syslog") != ""{
			hook, err := logrus_syslog.NewSyslogHook(
				"udp", c.String("syslog"), syslog.LOG_INFO, "")
			if err != nil {
				log.Error("Unable to connect to local syslog daemon")
			} else {
				log.AddHook(hook)
			}
		}

		log.SetOutput(os.Stderr)
		level, err := log.ParseLevel(c.String("log-level"))
		if err != nil {
			log.Fatalf(err.Error())
		}
		log.SetLevel(level)

		// If a log level wasn't specified and we are running in debug mode,
		// enforce log-level=debug.
		if !c.IsSet("log-level") && !c.IsSet("l") && c.Bool("debug") {
			log.SetLevel(log.DebugLevel)
		}

		if appBefore != nil {
			return appBefore(c)
		} else {
			return nil
		}
	}
}
