package log

/// `log` package of Raft-Go, inspired by Beego.

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var (
	defaultLogger = &RaftLogger{
		level:      LevelDebug,
		callDepth:  defaultCallDepth,
		msgChanLen: defaultAsyncMsgLen,
		outputs:    []*namedLogger{},
	}
	raftLogger = defaultLogger
)

const (
	defaultCallDepth   = 2
	defaultAsyncMsgLen = 1e3 // 1000

	timestampFormat = "2006-01-02 15:04:05.999999"
)

// Log level constants
const (
	LevelInfo = iota
	LevelDebug
	LevelWarning
	LevelError
	LevelFatal
	LevelPanic
)

var logLevelPrefix = map[int]string{
	LevelInfo:    "Info",
	LevelDebug:   "Debug",
	LevelWarning: "Warning",
	LevelError:   "Error",
	LevelFatal:   "Fatal",
	LevelPanic:   "Panic",
}

// Name for logger adapters
const (
	AdapterConsole = "console"
	AdapterFile    = "file"
	AdapterMail    = "mail"
	AdapterConnect = "connect"
)

type newLoggerFunc func() Logger

type Logger interface {
	Init(config string) error
	WriteMsg(level int, when time.Time, msg string) error
	Destroy()
	Flush()
}

// adapters for diff-kinds of logging output instance.
var adapters = make(map[string]newLoggerFunc)

// Register makes a log provider available by the provided name.
// Redundant registration will lead panic.
func Register(name string, loggerFunc newLoggerFunc) {
	if loggerFunc == nil {
		panic("log: Register provider is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("log: Named provider has existed, name: " + name)
	}
	adapters[name] = loggerFunc
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
	outputs    []*namedLogger
}

type namedLogger struct {
	Logger
	name string
}

type logMsg struct {
	msg   string
	level int
	when  time.Time
}

type logWriter struct {
	sync.Mutex
	writer io.Writer
}

func newLogWriter(wr io.Writer) *logWriter {
	return &logWriter{writer: wr}
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

func (rl *RaftLogger) SetLevel(level int) {
	rl.level = level
}

// Async enables asynchronous logging.
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

// startLogger is the concrete goroutine serves for async-logging.
// It receives msg to log or receives signal and reacts by what signal
// instructs, `flush` or `close`.
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

// SetLogger provides a registered logger into RaftLogger with config.
// Config should be JSON-format, and it will initialize logger in Init().
func (rl *RaftLogger) SetLogger(adapterName string, configs ...string) error {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	return rl.setLogger(adapterName, configs...)
}

// setLogger adds an output target for logging, it can be console, file, remote
// address...etc
func (rl *RaftLogger) setLogger(adapterName string, configs ...string) error {
	config := append(configs, "{}")[0]
	for _, l := range rl.outputs {
		if l.name == adapterName {
			return fmt.Errorf("log: redundant adapter name %s (you've set this logger before)", adapterName)
		}
	}
	logFunc, ok := adapters[adapterName]
	if !ok {
		return fmt.Errorf("log: unkown adapter name %s (please register it before)", adapterName)
	}
	lg := logFunc()
	err := lg.Init(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "log.SetLogger: "+err.Error())
		return err
	}
	rl.outputs = append(rl.outputs, &namedLogger{name: adapterName, Logger: lg})
	return nil
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

func (rl *RaftLogger) Write(msg []byte) (n int, err error) {
	if len(msg) == 0 {
		return 0, nil
	}
	// writeMsg will always add '\n'
	if msg[len(msg)-1] == '\n' {
		msg = msg[0 : len(msg)-1]
	}
	err = rl.writeMsg(LevelDebug, string(msg))
	if err == nil {
		return len(msg), err
	}
	return 0, nil
}

func (rl *RaftLogger) writeMsg(level int, msg string, v ...interface{}) error {
	if rl.async {
		lm := logMsgPool.Get().(*logMsg)
		lm.level = level
		lm.msg = logFormatter(level, msg, v...)
		lm.when = time.Now()
		rl.msgChan <- lm
	} else {
		lm := &logMsg{
			level: level,
			msg:   logFormatter(level, msg, v...),
			when:  time.Now(),
		}
		rl.writeToLoggers(lm)
	}
	return nil
}

func (rl *RaftLogger) Debug(format string, v ...interface{}) {
	if rl.level > LevelDebug {
		return
	}
	rl.writeMsg(LevelDebug, format, v...)
}

func (rl *RaftLogger) Info(format string, v ...interface{}) {
	if rl.level > LevelInfo {
		return
	}
	rl.writeMsg(LevelInfo, format, v...)
}

func (rl *RaftLogger) Warning(format string, v ...interface{}) {
	if rl.level > LevelWarning {
		return
	}
	rl.writeMsg(LevelWarning, format, v...)
}

func (rl *RaftLogger) Error(format string, v ...interface{}) {
	if rl.level > LevelError {
		return
	}
	rl.writeMsg(LevelError, format, v...)
}

func (rl *RaftLogger) Fatal(format string, v ...interface{}) {
	if rl.level > LevelFatal {
		return
	}
	rl.writeMsg(LevelFatal, format, v...)
}

func (rl *RaftLogger) Panic(format string, v ...interface{}) {
	if rl.level > LevelPanic {
		return
	}
	rl.writeMsg(LevelPanic, format, v...)
}

func logFormatter(level int, msg string, v ...interface{}) string {
	toLog := logLevelPrefix[level] + fmt.Sprintf(msg, v...)
	switch level {
	case LevelInfo:
		return coloredText(TextWhite, toLog)
	case LevelDebug:
		return coloredText(TextCyan, toLog)
	case LevelWarning:
		return coloredText(TextYellow, toLog)
	case LevelError:
		return coloredText(TextMagenta, toLog)
	case LevelFatal, LevelPanic:
		return coloredText(TextRed, toLog)
	default:
		return toLog
	}
}

func timeHeaderFormatter(time time.Time) string {
	return time.Format(timestampFormat)
}
