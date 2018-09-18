package src

import "time"

type SMTPLogWriter struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Subject  string `json:"subject"`
	FromAddr string `json:"fromAddr"`
	ToAddr   string `json:"toAddr"`
	Level    int    `json:"level"`
}

// Create new SMTPLogWriter returning as Logger
func newSMTPLogWriter() Logger {
	return &SMTPLogWriter{Level: LevelDebug}
}

func (s *SMTPLogWriter) Init(config string) error {

	return nil
}

func (s *SMTPLogWriter) WriteMsg(level int, when time.Time, msg string) error {
	return nil
}

func (s *SMTPLogWriter) Destroy() {

}

func (s *SMTPLogWriter) Flush() {

}
