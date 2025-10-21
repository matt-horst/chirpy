package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := time.Now()

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.RegisteredClaims {
			Issuer: "chirpy",
			IssuedAt: jwt.NewNumericDate(now), 
			ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
			Subject: userID.String(),
		}, 
	)

	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			return []byte(tokenSecret), nil
		},
	)

	if err != nil {
		return uuid.UUID{}, err
	}

	id, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.UUID{}, err
	}

	return uuid.Parse(id)
}

func GetBearerToken(headers http.Header) (string, error) {
	header := headers.Get("Authorization")
	if header == "" {
		return "", errors.New("No authorization header")
	}

	if !strings.HasPrefix(header, "Bearer") {
		return "", errors.New("No `Bearer` prefix found")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))

	if token == "" {
		return "", errors.New("No token found")
	}

	return token, nil
}

func MakeRefreshToken() (string, error) {
	bytes := [32]byte{}

	_, err := rand.Read(bytes[:])
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes[:]), nil
}
