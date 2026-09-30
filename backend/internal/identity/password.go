package identity

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id-Parameter nach OWASP-Empfehlung (ADR-015): m=19 MiB, t=2, p=1.
const (
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	minPassword  = 12
)

//go:embed common_passwords.txt
var commonPasswordsRaw string

var commonPasswords = func() map[string]struct{} {
	m := map[string]struct{}{}
	for _, l := range strings.Split(commonPasswordsRaw, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			m[strings.ToLower(l)] = struct{}{}
		}
	}
	return m
}()

// CheckPasswordPolicy prüft Mindestlänge und die Liste häufiger Passwörter (ID-03).
func CheckPasswordPolicy(pw string) error {
	if len([]rune(pw)) < minPassword {
		return fmt.Errorf("mindestens %d Zeichen", minPassword)
	}
	if _, ok := commonPasswords[strings.ToLower(pw)]; ok {
		return errors.New("zu häufig verwendetes Passwort")
	}
	return nil
}

// HashPassword erzeugt einen PHC-String ($argon2id$v=19$m=...,t=...,p=...$salt$hash).
func HashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword prüft ein Passwort gegen einen PHC-String in konstanter Zeit.
func VerifyPassword(pw, phc string) bool {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[4])
	want, err2 := enc.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash dient dazu, bei unbekannter E-Mail gleich lange zu rechnen.
var dummyHash, _ = HashPassword("vectra-dummy-password-for-timing")
