package log

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"strconv"
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
	MaxSize int `json:"maxSize"` // unit: MB
	curSize int
	// rotate daily
	Daily         bool  `json:"daily"`
	MaxDays       int32 `json:"maxDays"`
	dailyOpenDate int
	dailyOpenTime time.Time

	Rotate bool `json:"rotate"`

	Level int `json:"level"`

	Permission string `json:"permission"`
}

// Create a FileLogWriter instance returning as Logger with default config
func newFileWriter() Logger {
	return &fileLogWriter{
		Daily:      true,
		MaxDays:    7,
		Rotate:     true,
		Level:      LevelDebug,
		Permission: "0660", // only user and its group can read/write
	}
}

// Init file logger with json config.
// use it like:
//  {
//  "filename":"tmp/log.log",
//  "maxLines":10000,
//  "maxSize": 1024,
//  "daily":true,
//  "maxDays":7,
//  "rotate":true,
//  "permission":"0660"
//  }
func (w *fileLogWriter) Init(config string) error {
	err := json.Unmarshal([]byte(config), w)
	if err != nil {
		return err
	}
	if len(w.Filename) == 0 {
		return errors.New("json config must specify filename")
	}
	// append extension if not exist
	if filepath.Ext(w.Filename) == "" {
		w.Filename += ".log"
	}
	return w.startLogger()
}

func (w *fileLogWriter) WriteMsg(level int, when time.Time, msg string) error {
	return nil
}

func (w *fileLogWriter) Flush() {

}

func (w *fileLogWriter) Destroy() {

}

// create log file and init
func (w *fileLogWriter) startLogger() error {
	return nil
}

func (w *fileLogWriter) createLogFile() (*os.File, error) {
	perm, err := strconv.ParseInt(w.Permission, 8, 64)
	if err != nil {
		return nil, err
	}
	// according to Linux standard, do not forget O_WRONLY flag
	fd, err := os.OpenFile(w.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.FileMode(perm))
	if err == nil {
		// make sure permission is user set case of `os.OpenFile` will obey umask
		os.Chmod(w.Filename, os.FileMode(perm))
	}
	return fd, err
}

func (w *fileLogWriter) initFd() error {
	fd := w.fileWriter
	fInfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("get stat error: %s\n", err.Error())
	}
	w.curSize = int(fInfo.Size())
	w.dailyOpenTime = time.Now()
	w.dailyOpenDate = w.dailyOpenTime.Day()
	w.curLines = 0
	if w.Daily {

	}
	if fInfo.Size() > 0 {

	}
	return nil
}
