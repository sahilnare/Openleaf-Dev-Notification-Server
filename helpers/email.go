package helpers

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

const (
	B2B_EMAIL         = "b2b@openleaf.tech"
	APPOINTMENT_EMAIL = "appointment@openleaf.tech"
)

var host string
var port int = 587
var username string
var password string

func InitEmailConfig() {
	host = os.Getenv("EMAIL_HOST")
	username = os.Getenv("EMAIL_USERNAME")
	password = os.Getenv("EMAIL_PASSWORD")
	port, _ = strconv.Atoi(os.Getenv("EMAIL_PORT"))

	if host == "" || username == "" || password == ""  {
		LogException("email configuration incomplete", map[string]interface{}{
			"host":        host,
			"port":        port,
			"username":    username,
			"password_set": password != "",
		})
		return
	}

	LogInfo("email configuration initialized", map[string]interface{}{
		"host":        host,
		"port":        port,
		"username":    username,
	})
}

func SendEmail(from string, to []string, cc []string, subject string, body string, isHTML bool, files []string) error {

	if host == "" || username == "" || password == "" {
		return fmt.Errorf("email configuration incomplete: host=%s, port=%d, username=%s, password_set=%t",
			host, port, username, password != "")
	}

	m := gomail.NewMessage()

	m.SetHeader("From", from)
	m.SetHeader("To", to...)
	if len(cc) > 0 {
		m.SetHeader("Cc", cc...)
	}
	m.SetHeader("Subject", subject)

	if isHTML {
		m.SetBody("text/html", body)
	} else {
		m.SetBody("text/plain", body)
	}

	if len(files) > 0 {
		for _, file := range files {
			m.Attach(file)
		}
	}

	d := gomail.NewDialer(host, port, username, password)
	d.SSL = false
	d.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	// Log email configuration for debugging
	LogInfo("attempting to send email", map[string]interface{}{
		"host":        host,
		"port":        port,
		"username":    username,
		"from":        from,
		"to":          to,
		"cc":          cc,
		"subject":     subject,
	})

	if err := d.DialAndSend(m); err != nil {
		LogException("failed to send email", map[string]interface{}{
			"error":   err.Error(),
			"from":    from,
			"to":      to,
			"cc":      cc,
			"subject": subject,
			"body":    body,
			"isHTML":  isHTML,
			"files":   files,
		})
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil

}
