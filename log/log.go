package log

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

// Debugf thread safe write debug message
func Debugf(fmt string, args ...interface{}) {
	log.Debugf(fmt, args...)
}

// Warnf thread safe write warning message
func Warnf(fmt string, args ...interface{}) {
	log.Warnf(fmt, args...)
}

// Errorf thread safe write error message
func Errorf(fmt string, args ...interface{}) {
	log.Errorf(fmt, args...)
}

// SetLevel set logging level for flagger
func SetLevel(level logrus.Level) {
	log.SetLevel(level)
}
