package reminders

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/arran4/golang-ical"
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
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodPublish)
	cal.SetCalscale("GREGORIAN")
	cal.SetXWRCalName("ReAction Тестовый календарь")
	cal.SetXWRTimezone("UTC")
	cal.SetRefreshInterval("PT30S")

	cal.SetXPublishedTTL("PT30S")

	now := time.Now().UTC()
	event := cal.AddEvent(fmt.Sprintf("%s@reaction-test", now.Format("20060102T150405")))
	event.SetCreatedTime(now)
	event.SetDtStampTime(now)
	event.SetStartAt(now.Add(15 * time.Minute))
	event.SetEndAt(now.Add(20 * time.Minute))
	event.SetSummary("Тестовое событие для подписки")
	event.SetDescription("Это тестовое событие")
	event.SetLocation("Онлайн")

	alarm := event.AddAlarm()
	alarm.SetTrigger("-PT30M")
	alarm.SetAction(ics.ActionDisplay)
	alarm.SetDescription("Напоминание о тестовом событии")

	return cal.Serialize()
}
