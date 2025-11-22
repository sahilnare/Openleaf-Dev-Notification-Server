package helpers

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	host = os.Getenv("NOTIFICATION_SERVER_EMAIL_HOST")
	username = os.Getenv("NOTIFICATION_SERVER_EMAIL_USERNAME")
	password = os.Getenv("NOTIFICATION_SERVER_EMAIL_PASSWORD")
	port, _ = strconv.Atoi(os.Getenv("NOTIFICATION_SERVER_EMAIL_PORT"))

	if host == "" || username == "" || password == ""  {
		LogException("notification server email configuration incomplete", map[string]interface{}{
			"host":        host,
			"port":        port,
			"username":    username,
			"password_set": password != "",
		})
		return
	}

	LogInfo("notification server email configuration initialized", map[string]interface{}{
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

	// Download URL attachments to temporary files
	var tempFiles []string
	var fileNames []string
	var cleanupFunctions []func()

	cleanup := func() {
		for _, cleanupFunc := range cleanupFunctions {
			cleanupFunc()
		}
	}

	for i, url := range files {
		LogInfo("downloading attachment", map[string]interface{}{
			"url":   url,
			"index": i + 1,
			"total": len(files),
		})

		tempPath, fileName, cleanupFunc, err := DownloadURLToTempFile(url)
		if err != nil {
			LogException("failed to download attachment", map[string]interface{}{
				"url":   url,
				"index": i + 1,
				"error": err.Error(),
			})
			// Clean up any files already downloaded
			cleanup()
			return fmt.Errorf("failed to download attachment from %s: %w", url, err)
		}

		tempFiles = append(tempFiles, tempPath)
		fileNames = append(fileNames, fileName)
		cleanupFunctions = append(cleanupFunctions, cleanupFunc)

		LogInfo("downloaded attachment successfully", map[string]interface{}{
			"url":       url,
			"temp_path": tempPath,
			"index":     i + 1,
		})
	}

	if len(tempFiles) > 0 {
		for i, file := range tempFiles {
			m.Attach(file, gomail.SetHeader(map[string][]string{
				"Content-Disposition": {fmt.Sprintf(`attachment; filename="%s"`, fileNames[i])},
			}))
			// m.Attach(file)
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


func DownloadURLToTempFile(url string) (string, string, func(), error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// For local files, return the path and the filename
		return url, filepath.Base(url), func() {}, nil
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to download file from URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", nil, fmt.Errorf("HTTP error %d when downloading file from URL %s", resp.StatusCode, url)
	}

	filename := filepath.Base(url)
	if filename == "." || filename == "/" {
		filename = "attachment.pdf"
	}

	// Create temporary file
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, filename)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", "", nil, fmt.Errorf("failed to save downloaded file: %w", err)
	}

	tempFile.Close()

	cleanup := func() {
		os.Remove(tempFile.Name())
	}

	return tempFile.Name(), filename, cleanup, nil
}
