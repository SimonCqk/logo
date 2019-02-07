package logo

/// `logo` package of logo, inspired by Beego.

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

// Register makes a logo provider available by the provided name.
// Redundant registration will lead panic.
func Register(name string, loggerFunc newLoggerFunc) {
	if loggerFunc == nil {
		panic("logo: Register provider is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("logo: Named provider has existed, name: " + name)
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
	flushChan  chan struct{}
	closeChan  chan struct{}
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
		flushChan:  make(chan struct{}, 1),
		closeChan:  make(chan struct{}),
	}
	if rl.msgChanLen <= 0 {
		rl.msgChanLen = defaultAsyncMsgLen
	}
	return &rl
}

func (l *LogoLogger) EnableDebug() {
	l.level = LevelDebug
}

func (l *LogoLogger) SetLevel(level int) {
	l.level = level
}

// Async enables asynchronous logging.
func (l *LogoLogger) Async(msgLen int64) *LogoLogger {
	l.lock.Lock()
	defer l.lock.Unlock()
	if l.async {
		return l
	}
	l.async = true
	l.msgChanLen = msgLen
	l.msgChan = make(chan *logMsg, l.msgChanLen)
	logMsgPool = &sync.Pool{
		New: func() interface{} {
			return &logMsg{}
		},
	}
	l.wg.Add(1)
	go l.startLogger()
	return l
}

// startLogger is the concrete goroutine serves for async-logging.
// It receives msg to logo or receives signal and reacts by what signal
// instructs, `flush` or `close`.
func (l *LogoLogger) startLogger() {
	for {
		select {
		case msg := <-l.msgChan:
			l.writeToLoggers(msg)
			logMsgPool.Put(msg)
		case <-l.flushChan:
			l.flush()
			l.wg.Done()
		case <-l.closeChan:
			l.flush()
			for _, output := range l.outputs {
				output.Destroy()
			}
			l.outputs = nil
			l.wg.Done()
			return
		}
	}
}

// SetLogger provides a registered logger into LogoLogger with config.
// Config should be JSON-format, and it will initialize logger in Init().
func (l *LogoLogger) SetLogger(adapterName string, configs ...string) error {
	l.lock.Lock()
	defer l.lock.Unlock()
	if !l.init {
		l.outputs = []*namedLogger{}
		l.init = true
	}
	return l.setLogger(adapterName, configs...)
}

// Remove a logger adapter with given name, no error returned,
// if invalid adapterName passed in, then do nothing.
func (l *LogoLogger) DelLogger(adapterName string) {
	l.lock.Lock()
	defer l.lock.Unlock()
	idx := 0
	for _, lg := range l.outputs {
		if lg.name == adapterName {
			lg.Destroy()
			break
		}
		idx++
	}
	// remove logger by index, if no registered logger found, idx == len(l.outputs)
	if idx < len(l.outputs) {
		copy(l.outputs[idx:], l.outputs[idx+1:])
		l.outputs = l.outputs[:len(l.outputs)-1]
	}
}

// setLogger adds an output target for logging, it can be console, file, remote
// address...etc
func (l *LogoLogger) setLogger(adapterName string, configs ...string) error {
	config := append(configs, "{}")[0]
	for _, l := range l.outputs {
		if l.name == adapterName {
			return fmt.Errorf("logo: redundant adapter name %s (you've set this logger before)", adapterName)
		}
	}
	logFunc, ok := adapters[adapterName]
	if !ok {
		return fmt.Errorf("logo: unkown adapter name %s (please register it before)", adapterName)
	}
	lg := logFunc()
	err := lg.Init(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "logo.SetLogger: "+err.Error())
		return err
	}
	l.outputs = append(l.outputs, &namedLogger{name: adapterName, Logger: lg})
	return nil
}

func (l *LogoLogger) writeToLoggers(msg *logMsg) {
	for _, output := range l.outputs {
		err := output.WriteMsg(msg.level, msg.when, msg.msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write msg to %s, err is %s", output.name, err.Error())
		}
	}
}

func (l *LogoLogger) flush() {
	if l.async {
		for {
			// fetch all logo message and write to logger instantly.
			if len(l.msgChan) > 0 {
				m := <-l.msgChan
				l.writeToLoggers(m)
				logMsgPool.Put(m)
				continue
			}
			break
		}
	}
	for _, output := range l.outputs {
		output.Flush()
	}
}

func (l *LogoLogger) Flush() {
	if l.async {
		l.flushChan <- struct{}{}
		l.wg.Wait()
		l.wg.Add(1)
		return
	}
	l.flush()
}

func (l *LogoLogger) Close() {
	if l.async {
		l.closeChan <- struct{}{}
		l.wg.Wait()
		close(l.msgChan)
	} else {
		l.flush()
		for _, l := range l.outputs {
			l.Destroy()
		}
		l.outputs = nil
	}
	close(l.closeChan)
	close(l.flushChan)
}

func (l *LogoLogger) Reset() {
	l.Flush()
	for _, l := range l.outputs {
		l.Destroy()
	}
	l.outputs = nil
}

func (l *LogoLogger) Write(msg []byte) (n int, err error) {
	if len(msg) == 0 {
		return 0, nil
	}
	// writeMsg will always add '\n'
	if msg[len(msg)-1] == '\n' {
		msg = msg[0 : len(msg)-1]
	}
	err = l.writeMsg(LevelDebug, string(msg))
	if err == nil {
		return len(msg), err
	}
	return 0, nil
}

func (l *LogoLogger) writeMsg(level int, msg string, v ...interface{}) error {
	if !l.init {
		l.lock.Lock()
		l.setLogger(AdapterConsole)
		l.lock.Unlock()
	}
	if l.async {
		lm := logMsgPool.Get().(*logMsg)
		lm.level = level
		lm.msg = logLevelPrefix[level] + fmt.Sprintf(msg, v...)
		lm.when = time.Now()
		l.msgChan <- lm
	} else {
		lm := &logMsg{
			level: level,
			msg:   logLevelPrefix[level] + fmt.Sprintf(msg, v...),
			when:  time.Now(),
		}
		l.writeToLoggers(lm)
	}
	return nil
}

func (l *LogoLogger) Debug(format string, v ...interface{}) {
	if l.level > LevelDebug {
		return
	}
	l.writeMsg(LevelDebug, format, v...)
}

func (l *LogoLogger) Info(format string, v ...interface{}) {
	if l.level > LevelInfo {
		return
	}
	l.writeMsg(LevelInfo, format, v...)
}

func (l *LogoLogger) Warning(format string, v ...interface{}) {
	if l.level > LevelWarning {
		return
	}
	l.writeMsg(LevelWarning, format, v...)
}

func (l *LogoLogger) Error(format string, v ...interface{}) {
	if l.level > LevelError {
		return
	}
	l.writeMsg(LevelError, format, v...)
}

func (l *LogoLogger) Fatal(format string, v ...interface{}) {
	if l.level > LevelFatal {
		return
	}
	l.writeMsg(LevelFatal, format, v...)
}

func (l *LogoLogger) Panic(format string, v ...interface{}) {
	if l.level > LevelPanic {
		return
	}
	l.writeMsg(LevelPanic, format, v...)
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
