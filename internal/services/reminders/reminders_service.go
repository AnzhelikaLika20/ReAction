package reminders

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	secret []byte
}

func NewRemindersService(secret string) *Service {
	return &Service{secret: []byte(secret)}
}

func (s *Service) BuildCalendarURL(phone, baseURL string) string {
	phoneB64 := base64.URLEncoding.EncodeToString([]byte(phone))
	sig := s.signature(phone)
	return strings.TrimSuffix(baseURL, "/") + "/webcal/" + phoneB64 + "/" + sig + "/calendar.ics"
}

func (s *Service) VerifySignature(phoneBase64, signature string) (phone string, ok bool) {
	phoneBytes, err := base64.URLEncoding.DecodeString(phoneBase64)
	if err != nil {
		return "", false
	}
	phone = string(phoneBytes)
	expected := s.signature(phone)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return "", false
	}
	return phone, true
}

func (s *Service) signature(phone string) string {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(phone))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) GetCalendarICS(sessionID string) string {
	now := time.Now().UTC()
	eventStart := now.Add(15 * time.Minute)
	eventEnd := eventStart.Add(5 * time.Minute)
	dtFormat := "20060102T150405Z"

	events := fmt.Sprintf(`BEGIN:VEVENT
		UID:%s@reaction-test
		DTSTAMP:%s
		DTSTART:%s
		DTEND:%s
		SUMMARY:Тестовое событие для подписки
		DESCRIPTION:Это тестовое событие
		LOCATION:Онлайн
		BEGIN:VALARM
		TRIGGER:-PT30M
		ACTION:DISPLAY
		DESCRIPTION:Напоминание о тестовом событии
		END:VALARM
		END:VEVENT`,
		now.Format("20060102T150405"),
		now.Format(dtFormat),
		eventStart.Format(dtFormat),
		eventEnd.Format(dtFormat),
	)

	return fmt.Sprintf(`BEGIN:VCALENDAR
		VERSION:2.0
		PRODID:-//ReAction//Test Calendar//EN
		CALSCALE:GREGORIAN
		METHOD:PUBLISH
		X-WR-CALNAME:ReAction Тестовый календарь
		X-WR-TIMEZONE:UTC
		REFRESH-INTERVAL;VALUE=DURATION:PT30S
		X-PUBLISHED-TTL:PT30S
		%s
		END:VCALENDAR`, events)
}
