package log

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

var (
	defaultLogger = &DefaultLogger{Logger: log.New(os.Stderr, "raft", log.LstdFlags)}
	discardLogger = &DefaultLogger{Logger: log.New(ioutil.Discard, "", 0)}
	raftLogger    = Logger(defaultLogger)
)

const (
	callDepth = 2
)

type Logger interface {
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})

	Info(v ...interface{})
	Infof(format string, v ...interface{})

	Warning(v ...interface{})
	Warningf(format string, v ...interface{})

	Error(v ...interface{})
	Errorf(format string, v ...interface{})

	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})

	Panic(v ...interface{})
	Panicf(format string, v ...interface{})
}

// default impl of Logger interface
type DefaultLogger struct {
	*log.Logger
	isDebug bool
}

func (l *DefaultLogger) EnableTimestamps() {
	l.SetFlags(l.Flags() | log.Ldate | log.Ltime)
}

func (l *DefaultLogger) EnableDebug() {
	l.isDebug = true
}

func (l *DefaultLogger) Debug(v ...interface{}) {
	if l.isDebug {
		l.Output(callDepth, header("DEBUG", fmt.Sprint(v...)))
	}
}

func (l *DefaultLogger) Debugf(format string, v ...interface{}) {
	if l.isDebug {
		l.Output(callDepth, header("DEBUG", fmt.Sprintf(format, v)))
	}
}

func (l *DefaultLogger) Info(v ...interface{}) {

}

func (l *DefaultLogger) Infof(format string, v ...interface{}) {

}

func (l *DefaultLogger) Warning(v ...interface{}) {

}

func (l *DefaultLogger) Warningf(format string, v ...interface{}) {

}

func (l *DefaultLogger) Error(v ...interface{}) {

}

func (l *DefaultLogger) Errorf(format string, v ...interface{}) {

}

func (l *DefaultLogger) Fatal(v ...interface{}) {

}

func (l *DefaultLogger) Fatalf(format string, v ...interface{}) {

}

func (l *DefaultLogger) Panic(v ...interface{}) {

}

func (l *DefaultLogger) Panicf(format string, v ...interface{}) {

}

func header(val, msg string) string {
	return fmt.Sprintf("%s: %s", val, msg)
}
