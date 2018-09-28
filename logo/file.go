package logo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Impl of Logger Interface [disk file]
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

	fileNameOnly, suffix string
}

func init() {
	Register(AdapterFile, newFileWriter)
}

// Create new FileLogWriter returning as Logger with default config
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
//  "filename":"tmp/logo.logo",
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
	w.suffix = filepath.Ext(w.Filename)
	w.fileNameOnly = strings.TrimSuffix(w.Filename, w.suffix)
	// append extension if not exist
	if w.suffix == "" {
		w.suffix = ".logo"
	}
	return w.startLogger()
}

func (w *fileLogWriter) WriteMsg(level int, when time.Time, msg string) error {
	if level > w.Level {
		return nil
	}
	msg = timeHeaderFormatter(when) + msg + "\n"
	if w.Rotate {
		w.RLock()
		// double-check lock
		if w.needRotate(when.Day()) {
			w.RUnlock()
			w.Lock()
			if w.needRotate(when.Day()) {
				if err := w.doRotate(when); err != nil {
					fmt.Fprintf(os.Stderr, "FileLogWriter[%s]: %s\n", w.Filename, err.Error())
				}
			}
			w.Unlock()
		} else {
			w.RUnlock()
		}
	}
	w.Lock()
	_, err := w.fileWriter.Write([]byte(msg))
	if err == nil {
		w.curLines++
		w.curSize += len(msg)
	}
	w.Unlock()
	return nil
}

// commit current file context to stable disk, which means flush
// system-level disk buffer to real disk
func (w *fileLogWriter) Flush() {
	w.fileWriter.Sync()
}

func (w *fileLogWriter) Destroy() {
	w.fileWriter.Close()
}

// create logo file and init
func (w *fileLogWriter) startLogger() error {
	file, err := w.createLogFile()
	if err != nil {
		return err
	}
	// close the old one
	if w.fileWriter != nil {
		w.fileWriter.Close()
	}
	w.fileWriter = file
	return w.initFd()
}

func (w *fileLogWriter) createLogFile() (*os.File, error) {
	perm, err := strconv.ParseInt(w.Permission, 8, 64)
	if err != nil {
		return nil, err
	}
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
		go w.dailyRotate(w.dailyOpenTime)
	}
	if fInfo.Size() > 0 {
		lineCnt, err := w.lines()
		if err != nil {
			return err
		}
		w.curLines = lineCnt
	}
	return nil
}

func (w *fileLogWriter) dailyRotate(openTime time.Time) {
	// year, month, day of next day
	y, m, d := openTime.Add(24 * time.Hour).Date()
	nextDay := time.Date(y, m, d, 0, 0, 0, 0, openTime.Location())
	// add 100 nanosecond offset
	timer := time.NewTimer(time.Duration(nextDay.UnixNano() - openTime.UnixNano() + 100))
	select {
	case <-timer.C:
		w.Lock()
		if w.needRotate(time.Now().Day()) {
			if err := w.doRotate(time.Now()); err != nil {
				fmt.Fprintf(os.Stderr, "error in FileLogWriter[%s]: %s", w.Filename, err.Error())
			}
		}
		w.Unlock()
	}
}

func (w *fileLogWriter) needRotate(day int) bool {
	return (w.MaxLines > 0 && w.curLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.curSize >= w.MaxSize) ||
		(w.Daily && day != w.dailyOpenDate)
}

// doRotate means write new logo content to new file.
// new file named like xx.yyyy-mm-dd.logo (daily) or xx.001.logo (by line or size)
func (w *fileLogWriter) doRotate(now time.Time) error {
	// find the next available number
	num := 1
	var fName string
	_, err := os.Lstat(w.Filename)
	if err != nil {
		// file not exists or other, restart logger
		goto RestartLogger
	}

	if w.MaxLines > 0 || w.MaxSize > 0 {
		// by line or size enabled
		for ; err == nil && num <= 999; num++ {
			fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", now.Format("2006-01-02"), num, w.suffix)
			_, err = os.Lstat(fName)
		}
	} else {
		// by daily
		fName = fmt.Sprintf("%s.%s%s", w.fileNameOnly, w.dailyOpenTime.Format("2006-01-02"), w.suffix)
		_, err = os.Lstat(fName)
		for ; err == nil && num <= 999; num++ {
			fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", w.dailyOpenTime.Format("2006-01-02"), num, w.suffix)
			_, err = os.Lstat(fName)
		}
	}
	// the last file checked still exists
	if err == nil {
		return fmt.Errorf("Rotate: cannot find a free logo number to rename %s\n", w.Filename)
	}
	// close file before rename
	w.fileWriter.Close()
	err = os.Rename(w.Filename, fName)

	// restart always
RestartLogger:

	startLoggerErr := w.startLogger()
	go w.deleteOld()
	if startLoggerErr != nil {
		return fmt.Errorf("Rotate StartLogger: %s\n", startLoggerErr.Error())
	}
	if err != nil {
		return fmt.Errorf("Rotate: %s\n", err.Error())
	}
	return nil
}

func (w *fileLogWriter) deleteOld() {
	dir := filepath.Dir(w.Filename)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Unable to delete old logo file %s, error is: %s\n", path, r)
			}
		}()
		if info == nil {
			return nil
		}
		if !info.IsDir() &&
			info.ModTime().Add(24*time.Hour*time.Duration(w.MaxDays)).Before(time.Now()) &&
			strings.HasPrefix(filepath.Base(path), filepath.Base(w.fileNameOnly)) &&
			strings.HasSuffix(filepath.Base(path), w.suffix) {
			os.Remove(path)
		}
		return nil
	})
}

func (w *fileLogWriter) lines() (count int, err error) {
	// readonly
	fd, err := os.Open(w.Filename)
	if err != nil {
		return 0, err
	}
	defer fd.Close()
	// read 32k each time
	// according to verification, 4k~32k has a obvious performance gap,
	// while >32k the gap shrinks sharply, so 32k is a suitable size
	buf := make([]byte, 32*(1<<10))
	count = 0
	lineSep := []byte{'\n'}
	for {
		c, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			return count, err
		}
		count += bytes.Count(buf[:c], lineSep)
		if err == io.EOF {
			break
		}
	}
	return count, nil
}
