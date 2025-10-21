package auth

import (
	"net/http"
	"testing"
	"time"
	"fmt"

	"github.com/google/uuid"
)

func TestCheckPasswordHash(t *testing.T) {
	password1 := "password123"
	password2 := "guest"

	hash1, _ := HashPassword(password1)
	hash2, _ := HashPassword(password2)

	cases := [] struct {
		name string
		rawPassword string
		hashedPassword string
		expectError bool
		expectMatch bool
	} {
		{
			name: "Correct passord",
			rawPassword: password1,
			hashedPassword: hash1,
			expectError: false,
			expectMatch: true,
		},
		{
			name: "Incorrect password",
			rawPassword: password1,
			hashedPassword: hash2,
			expectError: false,
			expectMatch: false,
		},
		{
			name: "Invalid hash",
			rawPassword: password1,
			hashedPassword: "invalid hash",
			expectError: true,
			expectMatch: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := CheckPasswordHash(c.rawPassword, c.hashedPassword)
			if ((err != nil) != c.expectError) {
				t.Errorf("CheckPasswordHash() recieved error: %v, expected error: %v", err, c.expectError)
			}

			if (ok != c.expectMatch) {
				t.Errorf("CheckPasswordHash() recieved match: %v, expected match: %v", ok, c.expectMatch)
			}
		})
	}
}

func TestValidateJWT(t *testing.T) {
	userID := uuid.New()
	validToken, _ := MakeJWT(userID, "secret", time.Hour)
	shortToken, _ := MakeJWT(userID, "secret", time.Microsecond)

	cases := [] struct {
		name string
		tokenString string
		tokenSecret string
		delay bool
		expectedUserID uuid.UUID
		expectError bool
	} {
		{
			name: "Valid token",
			tokenString: validToken,
			tokenSecret: "secret",
			delay: false,
			expectedUserID: userID,
			expectError: false,
		},
		{
			name: "Invalid secret",
			tokenString: validToken,
			tokenSecret: "invalid secret",
			delay: false,
			expectedUserID: uuid.UUID{},
			expectError: true,
		},
		{
			name: "Invalid token",
			tokenString: "invalid token",
			tokenSecret: "secret",
			delay: false,
			expectedUserID: uuid.UUID{},
			expectError: true,
		},
		{
			name: "Expired token",
			tokenString: shortToken,
			tokenSecret: "secret",
			delay: false,
			expectedUserID: uuid.UUID{},
			expectError: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.delay {
				time.Sleep(time.Millisecond)
			}
			userID, err := ValidateJWT(c.tokenString, c.tokenSecret)
			if (err != nil) != c.expectError {
				t.Errorf("ValidateJWT() recieved error = %v, expects error = %v", err, c.expectError)
			}

			if (userID != c.expectedUserID) {
				t.Errorf("ValidateJWT() recieved userID = %v, expects userID = %v", userID, c.expectedUserID)
			}
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	tokenString := "token"

	validHeader := make(http.Header)
	validHeader.Add("Authorization", fmt.Sprintf("Bearer    %v", tokenString))

	missingAuthHeader := make(http.Header)

	missingBearerHeader := make(http.Header)
	missingBearerHeader.Add("Authorization", tokenString)

	cases := [] struct {
		name string
		header http.Header
		expectedToken string
		expectError bool
	} {
		{
			name: "Valid header",
			header: validHeader,
			expectedToken: "token",
			expectError: false,
		},
		{
			name: "Missing authentication key",
			header: missingAuthHeader,
			expectedToken: "",
			expectError: true,
		},
		{
			name: "Missing bearer token",
			header: missingBearerHeader,
			expectedToken: "",
			expectError: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bearerToken, err := GetBearerToken(c.header)
			if (err != nil) != c.expectError {
				t.Errorf("GetBearerToken() received error: %v, expected error: %v", err, c.expectError)
			}

			if bearerToken != c.expectedToken {
				t.Errorf("GetBearerToken() received token string: %v, expected token string: %v", bearerToken, c.expectedToken)
			}
		})
	}
}
