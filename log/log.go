package log

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

var (
	defaultLogger = &RaftLogger{Logger: log.New(os.Stderr, "raft", log.LstdFlags)}
	discardLogger = &RaftLogger{Logger: log.New(ioutil.Discard, "", 0)}
	raftLogger    = Logger(defaultLogger)
)

const (
	callDepth          = 3
	defaultAysncMsgLen = 1e3 // 1000
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

// RaftLogger is the default logger in Raft-Go
type RaftLogger struct {
	*log.Logger
	isDebug bool
}

func (l *RaftLogger) EnableTimestamps() {
	l.SetFlags(l.Flags() | log.Ldate | log.Ltime)
}

func (l *RaftLogger) EnableDebug() {
	l.isDebug = true
}

func (l *RaftLogger) Debug(v ...interface{}) {
	if l.isDebug {
		l.Output(callDepth, header("DEBUG", fmt.Sprint(v...)))
	}
}

func (l *RaftLogger) Debugf(format string, v ...interface{}) {
	if l.isDebug {
		l.Output(callDepth, header("DEBUG", fmt.Sprintf(format, v)))
	}
}

func (l *RaftLogger) Info(v ...interface{}) {
	l.Output(callDepth, header("Info", fmt.Sprint(v...)))
}

func (l *RaftLogger) Infof(format string, v ...interface{}) {
	l.Output(callDepth, header("Info", fmt.Sprintf(format, v...)))
}

func (l *RaftLogger) Warning(v ...interface{}) {
	l.Output(callDepth, header("Warning", fmt.Sprint(v...)))
}

func (l *RaftLogger) Warningf(format string, v ...interface{}) {
	l.Output(callDepth, header("Warning", fmt.Sprintf(format, v...)))
}

func (l *RaftLogger) Error(v ...interface{}) {
	l.Output(callDepth, header("Error", fmt.Sprint(v...)))
}

func (l *RaftLogger) Errorf(format string, v ...interface{}) {
	l.Output(callDepth, header("Error", fmt.Sprintf(format, v...)))
}

func (l *RaftLogger) Fatal(v ...interface{}) {
	l.Output(callDepth, header("Fatal", fmt.Sprint(v...)))
}

func (l *RaftLogger) Fatalf(format string, v ...interface{}) {
	l.Output(callDepth, header("Fatal", fmt.Sprintf(format, v...)))
}

func (l *RaftLogger) Panic(v ...interface{}) {
	l.Output(callDepth, header("Panic", fmt.Sprint(v...)))
}

func (l *RaftLogger) Panicf(format string, v ...interface{}) {
	l.Output(callDepth, header("Panic", fmt.Sprintf(format, v...)))
}

func header(val, msg string) string {
	return fmt.Sprintf("%s: %s", val, msg)
}
