package log

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
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
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warning(format string, v ...interface{})
	Error(format string, v ...interface{})
	Fatal(format string, v ...interface{})
	Panic(format string, v ...interface{})
}

// RaftLogger is the default logger in Raft-Go
type RaftLogger struct {
	*log.Logger
	lock       sync.Mutex
	debug      bool
	async      bool
	msgChanLen int64
	msgChan    chan *logMsg
	signalChan chan string
	wg         sync.WaitGroup
}

type logMsg struct {
	msg string
}

var logMsgPool *sync.Pool

func (rl *RaftLogger) EnableTimestamps() {
	rl.SetFlags(rl.Flags() | log.Ldate | log.Ltime)
}

func (rl *RaftLogger) EnableDebug() {
	rl.debug = true
}

func (rl *RaftLogger) Async(msgLen int64) *RaftLogger {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	if rl.async {
		return rl
	}
	rl.msgChanLen = msgLen
	rl.msgChan = make(chan *logMsg, rl.msgChanLen)
	logMsgPool = &sync.Pool{
		New: func() interface{} {
			return &logMsg{}
		},
	}
	rl.wg.Add(1)
	go rl.startLogger()
	return rl
}

func (rl *RaftLogger) startLogger() {
	exit := false
	for {
		select {
		case msg := <-rl.msgChan:
			rl.writeToLogger(msg.msg)
			logMsgPool.Put(msg)
		case sig := <-rl.signalChan:
			// signals include: `flush`, `close`
			rl.flush()
			if sig == "close" {
				exit = true
			}
			rl.wg.Done()
		}
		if exit {
			break
		}
	}
}

func (rl *RaftLogger) writeToLogger(msg string) {
	// TODO
}

func (rl *RaftLogger) flush() {
	if rl.async {
		for {
			// fetch all log message and write to logger.
			if len(rl.msgChan) > 0 {
				m := <-rl.msgChan
				rl.writeToLogger(m.msg)
				logMsgPool.Put(m)
				continue
			}
			break
		}
	}
	// TODO: maybe flush()
}

func (rl *RaftLogger) Debug(format string, v ...interface{}) {
	if rl.debug {
		rl.Output(callDepth, header("DEBUG", fmt.Sprintf(format, v)))
	}
}

func (rl *RaftLogger) Info(format string, v ...interface{}) {
	rl.Output(callDepth, header("Info", fmt.Sprintf(format, v...)))
}

func (rl *RaftLogger) Warning(format string, v ...interface{}) {
	rl.Output(callDepth, header("Warning", fmt.Sprintf(format, v...)))
}

func (rl *RaftLogger) Error(format string, v ...interface{}) {
	rl.Output(callDepth, header("Error", fmt.Sprintf(format, v...)))
}

func (rl *RaftLogger) Fatal(format string, v ...interface{}) {
	rl.Output(callDepth, header("Fatal", fmt.Sprintf(format, v...)))
}

func (rl *RaftLogger) Panic(format string, v ...interface{}) {
	rl.Output(callDepth, header("Panic", fmt.Sprintf(format, v...)))
}

func header(val, msg string) string {
	return fmt.Sprintf("%s: %s", val, msg)
}
