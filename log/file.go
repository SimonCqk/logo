package log

import (
	"os"
	"sync"
	"time"
)

// impl of Logger Interface
// It writes messages by lines limit, file size limit, or time frequency.
type fileLogWriter struct {
	sync.RWMutex
	Filename   string `json:"filename"`
	fileWriter *os.File
	// rotate at line
	MaxLines int `json:"maxLines"`
	curLines int
	// rotate at size
	MaxSize int `json:"maxSize"` // unit: byte
	curSize int
	// rotate daily
	Daily         bool  `json:"daily"`
	MaxDays       int32 `json:"maxDays"`
	dailyOpenDate int
	dailyOpenTime time.Time

	Level int `json:"level"`

	Permission string `json:"permission"`
}

// Create a FileLogWriter instance returning as Logger with default config
func newFileWriter() Logger {
	return &fileLogWriter{
		Daily:      true,
		MaxDays:    7,
		Level:      LevelDebug,
		Permission: "0666",
	}
}

func (w *fileLogWriter) Init(config string) error {
	return nil
}

func (w *fileLogWriter) WriteMsg(level int, when time.Time, msg string) error {
	return nil
}

func (w *fileLogWriter) Flush() {

}

func (w *fileLogWriter) Destroy() {

}
