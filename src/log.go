package src

/// `src` package of logo, inspired by Beego.

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var (
	defaultLogger = LogoLogger{
		level:      LevelDebug,
		callDepth:  defaultCallDepth,
		msgChanLen: defaultAsyncMsgLen,
		outputs:    []*namedLogger{},
	}
	logoLogger = &defaultLogger
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
	LevelInfo:    "[Info]",
	LevelDebug:   "[Debug]",
	LevelWarning: "[Warning]",
	LevelError:   "[Error]",
	LevelFatal:   "[Fatal]",
	LevelPanic:   "[Panic]",
}

// Name for logger adapters
const (
	AdapterConsole = "console"
	AdapterFile    = "file"
	AdapterMail    = "mail"
	AdapterConn    = "connect"
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

// Register makes a src provider available by the provided name.
// Redundant registration will lead panic.
func Register(name string, loggerFunc newLoggerFunc) {
	if loggerFunc == nil {
		panic("src: Register provider is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("src: Named provider has existed, name: " + name)
	}
	adapters[name] = loggerFunc
}

// LogoLogger is the default logger in logo
type LogoLogger struct {
	lock       sync.Mutex
	init       bool
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

func (lg *logWriter) println(when time.Time, msg string) {
	lg.Lock()
	h := []byte(timeHeaderFormatter(when))
	lg.writer.Write(append(append(h, msg...), '\n'))
	lg.Unlock()
}

var logMsgPool *sync.Pool

// NewLogger returns a new LogoLogger instance pointer.
// chanLen means the number of messages in chan(when async is true)
func NewLogger(chanLen ...int64) *LogoLogger {
	rl := LogoLogger{
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

func (rl *LogoLogger) EnableDebug() {
	rl.level = LevelDebug
}

func (rl *LogoLogger) SetLevel(level int) {
	rl.level = level
}

// Async enables asynchronous logging.
func (rl *LogoLogger) Async(msgLen int64) *LogoLogger {
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
// It receives msg to src or receives signal and reacts by what signal
// instructs, `flush` or `close`.
func (rl *LogoLogger) startLogger() {
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

// SetLogger provides a registered logger into LogoLogger with config.
// Config should be JSON-format, and it will initialize logger in Init().
func (rl *LogoLogger) SetLogger(adapterName string, configs ...string) error {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	if !rl.init {
		rl.outputs = []*namedLogger{}
		rl.init = true
	}
	return rl.setLogger(adapterName, configs...)
}

// Remove a logger adapter with given name, no error returned,
// if invalid adapterName passed in, then do nothing.
func (rl *LogoLogger) DelLogger(adapterName string) {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	idx := 0
	for _, lg := range rl.outputs {
		if lg.name == adapterName {
			lg.Destroy()
			break
		}
		idx++
	}
	// remove logger by index, if no registered logger found, idx == len(rl.outputs)
	if idx < len(rl.outputs) {
		copy(rl.outputs[idx:], rl.outputs[idx+1:])
		rl.outputs = rl.outputs[:len(rl.outputs)-1]
	}
}

// setLogger adds an output target for logging, it can be console, file, remote
// address...etc
func (rl *LogoLogger) setLogger(adapterName string, configs ...string) error {
	config := append(configs, "{}")[0]
	for _, l := range rl.outputs {
		if l.name == adapterName {
			return fmt.Errorf("src: redundant adapter name %s (you've set this logger before)", adapterName)
		}
	}
	logFunc, ok := adapters[adapterName]
	if !ok {
		return fmt.Errorf("src: unkown adapter name %s (please register it before)", adapterName)
	}
	lg := logFunc()
	err := lg.Init(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "src.SetLogger: "+err.Error())
		return err
	}
	rl.outputs = append(rl.outputs, &namedLogger{name: adapterName, Logger: lg})
	return nil
}

func (rl *LogoLogger) writeToLoggers(msg *logMsg) {
	for _, output := range rl.outputs {
		err := output.WriteMsg(msg.level, msg.when, msg.msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write msg to %s, err is %s", output.name, err.Error())
		}
	}
}

func (rl *LogoLogger) flush() {
	if rl.async {
		for {
			// fetch all src message and write to logger instantly.
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

func (rl *LogoLogger) Flush() {
	if rl.async {
		rl.signalChan <- "flush"
		rl.wg.Wait()
		rl.wg.Add(1)
		return
	}
	rl.flush()
}

func (rl *LogoLogger) Close() {
	if rl.async {
		rl.signalChan <- "close"
		rl.wg.Wait()
		close(rl.msgChan)
	} else {
		rl.flush()
		for _, l := range rl.outputs {
			l.Destroy()
		}
		rl.outputs = nil
	}
	close(rl.signalChan)
}

func (rl *LogoLogger) Reset() {
	rl.Flush()
	for _, l := range rl.outputs {
		l.Destroy()
	}
	rl.outputs = nil
}

func (rl *LogoLogger) Write(msg []byte) (n int, err error) {
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

func (rl *LogoLogger) writeMsg(level int, msg string, v ...interface{}) error {
	if !rl.init {
		rl.lock.Lock()
		rl.setLogger(AdapterConsole)
		rl.lock.Unlock()
	}
	if rl.async {
		lm := logMsgPool.Get().(*logMsg)
		lm.level = level
		lm.msg = logLevelPrefix[level] + fmt.Sprintf(msg, v...)
		lm.when = time.Now()
		rl.msgChan <- lm
	} else {
		lm := &logMsg{
			level: level,
			msg:   logLevelPrefix[level] + fmt.Sprintf(msg, v...),
			when:  time.Now(),
		}
		rl.writeToLoggers(lm)
	}
	return nil
}

func (rl *LogoLogger) Debug(format string, v ...interface{}) {
	if rl.level > LevelDebug {
		return
	}
	rl.writeMsg(LevelDebug, format, v...)
}

func (rl *LogoLogger) Info(format string, v ...interface{}) {
	if rl.level > LevelInfo {
		return
	}
	rl.writeMsg(LevelInfo, format, v...)
}

func (rl *LogoLogger) Warning(format string, v ...interface{}) {
	if rl.level > LevelWarning {
		return
	}
	rl.writeMsg(LevelWarning, format, v...)
}

func (rl *LogoLogger) Error(format string, v ...interface{}) {
	if rl.level > LevelError {
		return
	}
	rl.writeMsg(LevelError, format, v...)
}

func (rl *LogoLogger) Fatal(format string, v ...interface{}) {
	if rl.level > LevelFatal {
		return
	}
	rl.writeMsg(LevelFatal, format, v...)
}

func (rl *LogoLogger) Panic(format string, v ...interface{}) {
	if rl.level > LevelPanic {
		return
	}
	rl.writeMsg(LevelPanic, format, v...)
}

func GetLogoLogger() *LogoLogger {
	return logoLogger
}

// module level method-wrappers, actually op on default logger.
func Reset() {
	logoLogger.Reset()
}

func Async(msgLen ...int64) *LogoLogger {
	return logoLogger.Async(append(msgLen, defaultAsyncMsgLen)[0])
}

func SetLevel(level int) {
	logoLogger.SetLevel(level)
}

func SetLogger(adapterName string, config ...string) error {
	return logoLogger.setLogger(adapterName, config...)
}

func timeHeaderFormatter(time time.Time) string {
	return time.Format(timestampFormat)
}
