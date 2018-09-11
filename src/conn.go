package src

import (
	"encoding/json"
	"io"
	"net"
	"time"
)

// Impl of Logger Interface [network connection]
// It writes messages in keep-alive tcp connection.
type connLogWriter struct {
	lg          *logWriter
	innerWriter io.WriteCloser
	Reconnect   bool   `json:"reconnect"`
	Net         string `json:"net"`
	Addr        string `json:"addr"`
	Level       int    `json:"level"`
}

func init() {
	Register(AdapterConn, NewConnWriter)
}

// Create new ConnLogWriter returning as Logger
func NewConnWriter() Logger {
	conn := new(connLogWriter)
	conn.Level = LevelDebug
	return conn
}

// Init connection logger with json config.
func (c *connLogWriter) Init(config string) error {
	return json.Unmarshal([]byte(config), c)
}

// WriteMsg write message in connection.
// do re-connect if connection is down.
func (c *connLogWriter) WriteMsg(level int, when time.Time, msg string) error {
	if level > c.Level {
		return nil
	}
	if c.needToConnect() {
		err := c.connect()
		if err != nil {
			return err
		}
	}
	c.lg.println(when, msg)
	return nil
}

func (c *connLogWriter) Destroy() {
	if c.innerWriter != nil {
		c.innerWriter.Close()
	}
}

// Flush does nothing on connection.
func (c *connLogWriter) Flush() {

}

func (c *connLogWriter) connect() error {
	if c.innerWriter != nil {
		c.innerWriter.Close()
		c.innerWriter = nil
	}
	conn, err := net.Dial(c.Net, c.Addr)
	if err != nil {
		return err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
	}
	c.innerWriter = conn
	c.lg = newLogWriter(conn)
	return nil
}

func (c *connLogWriter) needToConnect() bool {
	if c.Reconnect {
		c.Reconnect = false
		return true
	}
	return c.innerWriter == nil
}
