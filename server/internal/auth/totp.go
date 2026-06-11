package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPIssuer is the issuer name shown in authenticator apps.
const TOTPIssuer = "Aegis"

// GenerateTOTP creates a new TOTP key for a user.
// Returns the OTP key which contains the secret and the provisioning URI for QR codes.
func GenerateTOTP(email string) (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: email,
	})
	if err != nil {
		return nil, fmt.Errorf("generate totp: %w", err)
	}
	return key, nil
}

// ValidateTOTP validates a 6-digit TOTP code against the secret.
func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateRecoveryCodes creates `count` random 8-character hex recovery codes.
// Returns plaintext codes — the caller is responsible for hashing before storage.
func GenerateRecoveryCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := range codes {
		b := make([]byte, 4) // 4 bytes = 8 hex chars
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generate recovery code: %w", err)
		}
		codes[i] = hex.EncodeToString(b)
	}
	return codes, nil
}
