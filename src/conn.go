package src

import (
	"io"
	"time"
)

// Impl of Logger Interface [network connection]
// It writes messages in keep-alive tcp connection.
type connLogWriter struct {
	lw          *logWriter
	innerWriter io.WriteCloser

	Net   string `json:"net"`
	Addr  string `json:"addr"`
	Level int    `json:"level"`
}

func init() {
	Register(AdapterConnect, NewConnWriter)
}

// Create new ConnLogWriter returning as Logger
func NewConnWriter() Logger {
	conn := new(connLogWriter)
	conn.Level = LevelDebug
	return conn
}

func (c *connLogWriter) Init(config string) error {
	return nil
}

func (c *connLogWriter) WriteMsg(level int, when time.Time, msg string) error {
	return nil
}

func (c *connLogWriter) Destroy() {

}

func (c *connLogWriter) Flush() {

}
