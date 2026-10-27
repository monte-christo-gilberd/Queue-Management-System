package joinlink_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"qms/qr-service/internal/joinlink"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func newTestSigner(t *testing.T) *joinlink.Signer {
	t.Helper()
	s, err := joinlink.New(testSecret)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

// sign menghitung tanda tangan HMAC-SHA256 dengan format yang sama seperti
// Signer.sign. Dipakai untuk menyusun token "valid secara tanda tangan" tetapi
// isinya sengaja salah, sehingga tes tetap black-box.
func sign(secret, signingInput string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestNew_RejectsShortSecret(t *testing.T) {
	if _, err := joinlink.New("short"); err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestIssueStatic_VerifiesAndHasNoExpiry(t *testing.T) {
	s := newTestSigner(t)

	token, issued, err := s.Issue(12, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if issued.ExpiresAt != 0 {
		t.Fatalf("static token must not have exp, got %d", issued.ExpiresAt)
	}

	claims, err := s.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.QueueID != 12 || claims.Type != joinlink.TypeStatic {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.JTI == "" {
		t.Fatal("jti must not be empty")
	}
	if claims.ExpiresAt != 0 {
		t.Fatalf("static claims must not have exp, got %d", claims.ExpiresAt)
	}
}

func TestIssueDynamic_HasExpiryWithinTTL(t *testing.T) {
	s := newTestSigner(t)
	before := time.Now().Add(2 * time.Minute).Unix()

	token, issued, err := s.Issue(7, joinlink.TypeDynamic, 2*time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	after := time.Now().Add(2 * time.Minute).Unix()
	if issued.ExpiresAt < before-1 || issued.ExpiresAt > after+1 {
		t.Fatalf("exp %d outside expected window [%d,%d]", issued.ExpiresAt, before, after)
	}

	claims, err := s.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Type != joinlink.TypeDynamic || claims.ExpiresAt != issued.ExpiresAt {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestIssueDynamic_DefaultTTLWhenZero(t *testing.T) {
	s := newTestSigner(t)
	_, issued, err := s.Issue(1, joinlink.TypeDynamic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	want := time.Now().Add(joinlink.DefaultDynamicTTL).Unix()
	if diff := issued.ExpiresAt - want; diff < -2 || diff > 2 {
		t.Fatalf("expected exp near %d, got %d", want, issued.ExpiresAt)
	}
}

func TestIssue_RotationProducesDifferentTokens(t *testing.T) {
	s := newTestSigner(t)

	first, _, err := s.Issue(5, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	second, _, err := s.Issue(5, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if first == second {
		t.Fatal("expected rotation to produce a different token")
	}
}

func TestIssue_RejectsInvalidInput(t *testing.T) {
	s := newTestSigner(t)

	if _, _, err := s.Issue(0, joinlink.TypeStatic, 0); !errors.Is(err, joinlink.ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload for queue 0, got %v", err)
	}
	if _, _, err := s.Issue(1, joinlink.Type("other"), 0); !errors.Is(err, joinlink.ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload for unknown type, got %v", err)
	}
}

func TestVerify_ExpiredDynamic(t *testing.T) {
	s := newTestSigner(t)
	token, _, err := s.Issue(3, joinlink.TypeDynamic, time.Millisecond)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	if _, err := s.Verify(token); !errors.Is(err, joinlink.ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	s := newTestSigner(t)
	token, _, err := s.Issue(3, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	parts := strings.Split(token, ".")
	forged := base64.RawURLEncoding.EncodeToString([]byte(`{"qid":99,"typ":"static","jti":"abc"}`))
	tampered := parts[0] + "." + forged + "." + parts[2]

	if _, err := s.Verify(tampered); !errors.Is(err, joinlink.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestVerify_TamperedSignature(t *testing.T) {
	s := newTestSigner(t)
	token, _, err := s.Issue(3, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	parts := strings.Split(token, ".")
	sig := []byte(parts[2])
	sig[0] ^= 0xff
	tampered := parts[0] + "." + parts[1] + "." + string(sig)

	if _, err := s.Verify(tampered); !errors.Is(err, joinlink.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	issuer := newTestSigner(t)
	other, err := joinlink.New("abcdef0123456789abcdef0123456789")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	token, _, err := issuer.Issue(3, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := other.Verify(token); !errors.Is(err, joinlink.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestVerify_Malformed(t *testing.T) {
	s := newTestSigner(t)

	cases := []string{
		"",
		"not-a-token",
		"v1.onlytwo",
		"v1.!!!.sig",
		"v1." + base64.RawURLEncoding.EncodeToString([]byte("{}")) + ".sig",
		"v2." + base64.RawURLEncoding.EncodeToString([]byte("{}")) + ".sig",
		strings.Repeat("a", joinlink.MaxTokenLength+1),
	}

	for _, token := range cases {
		if _, err := s.Verify(token); err == nil {
			t.Fatalf("expected error for %q", token)
		}
	}
}

func TestVerify_UnsupportedVersion(t *testing.T) {
	s := newTestSigner(t)
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"qid":1,"typ":"static","jti":"x"}`))
	input := "v2." + payload
	token := input + "." + sign(testSecret, input)

	if _, err := s.Verify(token); !errors.Is(err, joinlink.ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestVerify_DynamicWithoutExpiryIsInvalid(t *testing.T) {
	s := newTestSigner(t)
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"qid":1,"typ":"dynamic","jti":"x"}`))
	input := joinlink.Version + "." + payload
	token := input + "." + sign(testSecret, input)

	if _, err := s.Verify(token); !errors.Is(err, joinlink.ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestVerify_MissingJTI(t *testing.T) {
	s := newTestSigner(t)
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"qid":1,"typ":"static"}`))
	input := joinlink.Version + "." + payload
	token := input + "." + sign(testSecret, input)

	if _, err := s.Verify(token); !errors.Is(err, joinlink.ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}
