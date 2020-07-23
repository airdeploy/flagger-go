package log

import (
	"github.com/sirupsen/logrus"
)

// Debugf thread safe write debug message
func Debugf(fmt string, args ...interface{}) {
	logrus.Debugf(fmt, args...)
}

// Warnf thread safe write warning message
func Warnf(fmt string, args ...interface{}) {
	logrus.Warnf(fmt, args...)
}

// Errorf thread safe write error message
func Errorf(fmt string, args ...interface{}) {
	logrus.Errorf(fmt, args...)
}
