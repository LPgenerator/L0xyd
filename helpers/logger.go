package helpers

import (
	log "github.com/Sirupsen/logrus"
)

type OxyLogger struct {
}

func (oxylogger *OxyLogger) Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

func (oxylogger *OxyLogger) Warningf(format string, args ...interface{}) {
	log.Warningf(format, args...)
}

func (oxylogger *OxyLogger) Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}
