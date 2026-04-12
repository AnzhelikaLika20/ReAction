package mail

import (
	"fmt"
	"mime"
	"net/smtp"
	"strings"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

type Client struct {
	cfg Config
}

func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) Configured() bool {
	return strings.TrimSpace(c.cfg.Host) != ""
}

func (c *Client) SendRegistrationVerification(toEmail, verificationURL string) error {
	if !c.Configured() {
		return nil
	}
	port := c.cfg.Port
	if port == "" {
		port = "587"
	}
	from := strings.TrimSpace(c.cfg.From)
	if from == "" {
		return fmt.Errorf("mail: empty From address")
	}
	host := strings.TrimSpace(c.cfg.Host)
	addr := fmt.Sprintf("%s:%s", host, port)

	subject := mime.BEncoding.Encode("UTF-8", "Re:Action — подтверждение email")
	body := fmt.Sprintf(`Здравствуйте.

Для завершения регистрации перейдите по ссылке (действует ограниченное время):

%s

Если вы не регистрировались в Re:Action, проигнорируйте это письмо.
`, verificationURL)

	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s",
		from, toEmail, subject, body))

	auth := smtp.PlainAuth("", strings.TrimSpace(c.cfg.User), c.cfg.Password, host)
	return smtp.SendMail(addr, auth, from, []string{toEmail}, msg)
}
