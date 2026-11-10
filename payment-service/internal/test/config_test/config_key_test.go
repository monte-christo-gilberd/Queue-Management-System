package config_test

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qms/payment-service/internal/config"
)

func writeKeyFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadWithKey(t *testing.T, path string) error {
	t.Helper()
	setBase(t)
	t.Setenv("JWT_PUBLIC_KEY_FILE", path)
	_, err := config.Load()
	return err
}

func TestPublicKeyFileMissing(t *testing.T) {
	err := loadWithKey(t, filepath.Join(t.TempDir(), "nope.pem"))
	if err == nil || !strings.Contains(err.Error(), "JWT_PUBLIC_KEY_FILE") {
		t.Fatalf("err = %v", err)
	}
}

func TestPublicKeyNotPEM(t *testing.T) {
	err := loadWithKey(t, writeKeyFile(t, "bad.pem", []byte("this is not pem")))
	if err == nil || !strings.Contains(err.Error(), "not a PEM file") {
		t.Fatalf("err = %v", err)
	}
}

func TestPublicKeyBadDER(t *testing.T) {
	data := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("garbage")})
	err := loadWithKey(t, writeKeyFile(t, "der.pem", data))
	if err == nil || !strings.Contains(err.Error(), "parse public key") {
		t.Fatalf("err = %v", err)
	}
}

func TestPublicKeyNotRSA(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	err = loadWithKey(t, writeKeyFile(t, "ed.pem", data))
	if err == nil || !strings.Contains(err.Error(), "not RSA") {
		t.Fatalf("err = %v", err)
	}
}
