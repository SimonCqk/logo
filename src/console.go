package src

import (
	"os"
	"time"
)

type consoleLogWriter struct {
	lg    *logWriter
	Level int  `json:"level"`
	Color bool `json:"color"`
}

func init() {
	Register(AdapterConsole, NewConsoleWriter)
}

func NewConsoleWriter() Logger {
	return &consoleLogWriter{
		lg:    newLogWriter(os.Stdout),
		Level: LevelDebug,
		Color: isWindows(),
	}
}

func (c *consoleLogWriter) Init(config string) error {
	return nil
}

func (c *consoleLogWriter) WriteMsg(level int, when time.Time, msg string) error {
	return nil
}

func (c *consoleLogWriter) Flush() {

}

func (c *consoleLogWriter) Destroy() {

}
