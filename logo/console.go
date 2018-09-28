package logo

import (
	"encoding/json"
	"os"
	"runtime"
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

// Init console logger with json config.
// use it like:
// {
// "level": LevelDebug,
// "color": true
// }
func (c *consoleLogWriter) Init(config string) error {
	if len(config) == 0 {
		c.Level = LevelDebug
		c.Color = isWindows()
		return nil
	}
	err := json.Unmarshal([]byte(config), c)
	if runtime.GOOS == "windows" {
		c.Color = false
	}
	return err
}

// WriteMsg write message in console.
func (c *consoleLogWriter) WriteMsg(level int, when time.Time, msg string) error {
	if level > c.Level {
		return nil
	}
	if c.Color {
		msg = coloredText(level, msg)
	}
	c.lg.println(when, msg)
	return nil
}

// Flush impl method and does nothing.
func (c *consoleLogWriter) Flush() {

}

// Destroy impl method and does nothing.
func (c *consoleLogWriter) Destroy() {

}
