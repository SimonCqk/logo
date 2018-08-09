package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

var (
	defaultLogger = &RaftLogger{Logger: log.New(os.Stderr, "raft", log.LstdFlags)}
	raftLogger    = Logger(defaultLogger)
)

const (
	defaultCallDepth   = 2
	defaultAsyncMsgLen = 1e3 // 1000
)

// log level constants
const (
	LevelInfo = iota
	LevelDebug
	LevelWarning
	LevelError
	LevelFatal
	LevelPanic
)

var logLevelStr = map[int]string{
	LevelInfo:    "Info",
	LevelDebug:   "Debug",
	LevelWarning: "Warning",
	LevelError:   "Error",
	LevelFatal:   "Fatal",
	LevelPanic:   "Panic",
}

type Logger interface {
	io.WriteCloser

	WriteMsg(level int, when time.Time, msg string) error
	Destroy()
	Flush()

	Info(format string, v ...interface{})
	Debug(format string, v ...interface{})
	Warning(format string, v ...interface{})
	Error(format string, v ...interface{})
	Fatal(format string, v ...interface{})
	Panic(format string, v ...interface{})
}

// RaftLogger is the default logger in Raft-Go
type RaftLogger struct {
	lock       sync.Mutex
	async      bool
	level      int
	callDepth  int
	msgChanLen int64
	msgChan    chan *logMsg
	signalChan chan string
	wg         sync.WaitGroup
	outputs    []*nameLogger
}

type nameLogger struct {
	Logger
	name string
}

type logMsg struct {
	msg   string
	level int
	when  time.Time
}

var logMsgPool *sync.Pool

// NewLogger returns a new RaftLogger instance pointer.
// chanLen means the number of messages in chan(when async is true)
func NewLogger(chanLen ...int64) *RaftLogger {
	rl := RaftLogger{
		level:      LevelDebug,
		callDepth:  defaultCallDepth,
		msgChanLen: append(chanLen, 0)[0],
		signalChan: make(chan string, 1),
	}
	if rl.msgChanLen <= 0 {
		rl.msgChanLen = defaultAsyncMsgLen
	}
	return &rl
}

func (rl *RaftLogger) EnableDebug() {
	rl.level = LevelDebug
}

func (rl *RaftLogger) Async(msgLen int64) *RaftLogger {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	if rl.async {
		return rl
	}
	rl.async = true
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
			rl.writeToLoggers(msg)
			logMsgPool.Put(msg)
		case sig := <-rl.signalChan:
			// signals include: `flush`, `close`
			rl.flush()
			if sig == "close" {
				for _, output := range rl.outputs {
					output.Destroy()
				}
				rl.outputs = nil
				exit = true
			}
			rl.wg.Done()
		}
		if exit {
			break
		}
	}
}

func (rl *RaftLogger) writeToLoggers(msg *logMsg) {
	for _, output := range rl.outputs {
		err := output.WriteMsg(msg.level, msg.when, msg.msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write msg to %s, err is %s", output.name, err.Error())
		}
	}
}

func (rl *RaftLogger) flush() {
	if rl.async {
		for {
			// fetch all log message and write to logger instantly.
			if len(rl.msgChan) > 0 {
				m := <-rl.msgChan
				rl.writeToLoggers(m)
				logMsgPool.Put(m)
				continue
			}
			break
		}
	}
	for _, output := range rl.outputs {
		output.Flush()
	}
}

/*
func (rl *RaftLogger) Debug(format string, v ...interface{}) {
	if rl.debug {
		rl.Output(defaultCallDepth, header("DEBUG", fmt.Sprintf(format, v)))
	}
}

func (rl *RaftLogger) Info(format string, v ...interface{}) {
	rl.Output(defaultCallDepth, header("Info", fmt.Sprintf(format, v...)))
}

func (rl *RaftLogger) Warning(format string, v ...interface{}) {
	rl.Output(defaultCallDepth, header("Warning", fmt.Sprintf(format, v...)))
}

func (rl *RaftLogger) Error(format string, v ...interface{}) {
	rl.Output(defaultCallDepth, header("Error", fmt.Sprintf(format, v...)))
}

func (rl *RaftLogger) Fatal(format string, v ...interface{}) {
	rl.Output(defaultCallDepth, header("Fatal", fmt.Sprintf(format, v...)))
}

func (rl *RaftLogger) Panic(format string, v ...interface{}) {
	rl.Output(defaultCallDepth, header("Panic", fmt.Sprintf(format, v...)))
}
*/

func logFormatter(level int, format string, msg ...interface{}) string {
	return logLevelStr[level] + fmt.Sprintf(format, msg...)
}
