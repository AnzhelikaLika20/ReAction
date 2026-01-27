package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	SessionID   string `json:"session_id"`
	PhoneNumber string `json:"phone_number"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secretKey     []byte
	tokenDuration time.Duration
}

func NewJWTService(
	secretKey string,
	tokenDuration time.Duration,
) *JWTService {
	return &JWTService{
		secretKey:     []byte(secretKey),
		tokenDuration: tokenDuration,
	}
}

func (s *JWTService) GenerateToken(phoneNumber string) (string, error) {
	// TODO: generate ID
	generatedId, _ := uuid.NewUUID()
	// generatedId := "e3f766fd-f2f4-11f0-ab19-e627c684288d"
	claims := &Claims{
		PhoneNumber: phoneNumber,
		SessionID:   generatedId.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   phoneNumber,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func (s *JWTService) GetTokenDuration() time.Duration {
	return s.tokenDuration
}
