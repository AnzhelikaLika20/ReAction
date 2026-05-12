package auth

import (
	"errors"
	"unicode"
	"unicode/utf8"
)

var ErrWeakPassword = errors.New(
	"Пароль должен быть не короче 8 символов и содержать строчные и заглавные буквы и хотя бы одну цифру.",
)

const minCredentialPasswordRunes = 8

func ValidateCredentialPassword(password string) error {
	if utf8.RuneCountInString(password) < minCredentialPasswordRunes {
		return ErrWeakPassword
	}
	var hasLower, hasUpper, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLower || !hasUpper || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}
