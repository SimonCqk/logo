package logo

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type SMTPLogWriter struct {
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	Host        string   `json:"host"`
	Subject     string   `json:"subject"`
	FromAddress string   `json:"fromAddress"`
	ToAddresses []string `json:"toAddresses"`
	Level       int      `json:"level"`
}

func init() {
	Register(AdapterMail, newSMTPLogWriter)
}

// Create new SMTPLogWriter returning as Logger
func newSMTPLogWriter() Logger {
	return &SMTPLogWriter{Level: LevelDebug}
}

// Init smtp logger with json config.
// use it like:
//  {
//  "username":"example@gmail.com",
//  "password":"password",
//  "host":"smtp.gmail.com:465",
//  "subject":"email title",
//  "fromAddr":"from@example.com",
//  "toAddr":["email1","email2"],
//  "level":LevelDebug
//  }
func (s *SMTPLogWriter) Init(config string) error {
	err := json.Unmarshal([]byte(config), s)
	return err
}

func (s *SMTPLogWriter) getSMTPAuth(host string) smtp.Auth {
	if len(strings.Trim(s.Username, " ")) == 0 || len(strings.Trim(s.Password, " ")) == 0 {
		return nil
	}
	return smtp.PlainAuth("", s.Username, s.Password, host)
}

func (s *SMTPLogWriter) sendMail(
	hostWithPort string,
	auth smtp.Auth,
	fromAddress string,
	toAddresses []string,
	msg []byte) error {
	client, err := smtp.Dial(hostWithPort)
	if err != nil {
		return err
	}
	host, _, _ := net.SplitHostPort(hostWithPort)
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}
	if err = client.StartTLS(tlsConf); err != nil {
		return err
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(fromAddress); err != nil {
		return err
	}
	for _, toAddr := range toAddresses {
		if err = client.Rcpt(toAddr); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	err = client.Quit()
	return err
}

func (s *SMTPLogWriter) WriteMsg(level int, when time.Time, msg string) error {
	if level > s.Level {
		return nil
	}
	hp := strings.Split(s.Host, ":")
	// set up authentication
	auth := s.getSMTPAuth(hp[0])
	// connect to server, authenticate, set the sender and recipient,
	// and set the email all in one step
	contentType := "Content-Type: text/plain" + "; charset=UTF-8"
	toMail := []byte("To: " + strings.Join(s.ToAddresses, ";") + "\r\nFrom: " + s.FromAddress + "<" + s.FromAddress +
		">\r\nSubject: " + s.Subject + "\r\n" + contentType + "\r\n\r\n" + fmt.Sprintf(".%s",
		when.Format("2006-01-02 15:04:05")) + msg)
	return s.sendMail(s.Host, auth, s.FromAddress, s.ToAddresses, toMail)
}

// Destroy do nothing.
func (s *SMTPLogWriter) Destroy() {
	return
}

// Flush do nothing.
func (s *SMTPLogWriter) Flush() {
	return
}
