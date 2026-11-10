package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"qms/auth-service/internal/apperror"
)

const (
	// maxUsernameLen: batas panjang username dalam karakter (rune), sama dengan backend.
	maxUsernameLen = 50
	// maxLinkAttempts: salah password yang diizinkan per link token sebelum token dibakar.
	maxLinkAttempts = 5
)

// GoogleIdentity: identitas dari ID token Google yang sudah diperiksa (tanda tangan, issuer, audience, masa berlaku,
// dan email_verified). Hanya ini yang boleh dipercaya; tidak ada data dari klien.
type GoogleIdentity struct {
	// Subject: id akun Google yang stabil (klaim `sub`). Dipakai untuk mencocokkan akun, bukan email (FR 4.17).
	Subject string
	// Email: email akun Google, sudah diverifikasi Google.
	Email string
	// Name: nama tampilan di akun Google; boleh kosong.
	Name string
}

// GoogleVerifier memeriksa ID token Google. Dipenuhi google.Verifier; di tes diganti fake.
type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (GoogleIdentity, error)
}

// LinkRequiredError dikembalikan LoginWithGoogle bila email sudah terdaftar dengan password. Membawa link token
// untuk dikirim kembali bersama password di LinkGoogle. Cocok dengan apperror.ErrGoogleLinkRequired lewat errors.Is.
type LinkRequiredError struct {
	// Token: link token (JWT RS256, 5 menit, purpose google_link).
	Token string
}

func (e *LinkRequiredError) Error() string { return apperror.ErrGoogleLinkRequired.Error() }

// Is membuat errors.Is(err, apperror.ErrGoogleLinkRequired) bernilai true.
func (e *LinkRequiredError) Is(target error) bool { return target == apperror.ErrGoogleLinkRequired }

// WithGoogle memasang pemeriksa ID token Google dan menyalakan login Google.
func (s *Service) WithGoogle(v GoogleVerifier) *Service {
	s.google = v
	return s
}

// GoogleEnabled melaporkan apakah login Google dinyalakan.
func (s *Service) GoogleEnabled() bool { return s.google != nil }

// LoginWithGoogle memeriksa ID token lalu:
//  1. akun yang tertaut ke google_id ini ada: terbitkan token login;
//  2. email sudah dipakai akun lain: JANGAN tautkan otomatis. Kembalikan *LinkRequiredError (link token) supaya user
//     membuktikan password akun itu lebih dulu (pengambilalihan akun lewat email dicegah);
//  3. email belum ada: buat akun baru tanpa password, emailnya terverifikasi (Google sudah menjaminnya).
func (s *Service) LoginWithGoogle(ctx context.Context, idToken string) (string, error) {
	if s.google == nil {
		return "", apperror.ErrGoogleUnavailable
	}
	ident, err := s.google.Verify(ctx, idToken)
	if err != nil {
		return "", err
	}

	// Dua kali percobaan: pendaftaran serentak untuk akun Google yang sama bisa menabrak UNIQUE di CreateGoogle.
	for attempt := 0; attempt < 2; attempt++ {
		// 1. Sudah tertaut: cocokkan lewat google_id.
		user, err := s.users.FindByGoogleID(ctx, ident.Subject)
		if err == nil {
			return s.issuer.Issue(user.ID)
		}
		if !errors.Is(err, apperror.ErrNotFound) {
			return "", err
		}

		// 2. Email sudah terdaftar: perlu konfirmasi password.
		existing, err := s.users.FindAccountByEmail(ctx, ident.Email)
		switch {
		case err == nil:
			return "", s.linkRequired(existing.ID, existing.PasswordHash.Valid, existing.GoogleID.Valid, ident.Subject)
		case !errors.Is(err, apperror.ErrNotFound):
			return "", err
		}

		// 3. Email baru: buat akun Google.
		created, err := s.users.CreateGoogle(ctx, ident.Email, usernameFor(ident), ident.Subject)
		if err == nil {
			return s.issuer.Issue(created.ID)
		}
		// Duplikat: ada yang membuat akun itu bersamaan; ulangi keputusan dari awal.
		if !errors.Is(err, apperror.ErrDuplicate) {
			return "", err
		}
	}
	return "", apperror.ErrDuplicate
}

// linkRequired membuat galat konfirmasi password untuk akun yang sudah ada. Akun yang sudah tertaut ke akun Google
// lain, atau tidak punya password untuk dibuktikan, tidak bisa ditautkan: ErrDuplicate (409 CONFLICT).
func (s *Service) linkRequired(userID int64, hasPassword, hasGoogle bool, googleID string) error {
	if hasGoogle || !hasPassword {
		return apperror.ErrDuplicate
	}
	token, err := s.issuer.IssueLink(userID, googleID)
	if err != nil {
		return err
	}
	return &LinkRequiredError{Token: token}
}

// LinkGoogle menautkan akun Google setelah user membuktikan password akunnya, lalu menerbitkan token login.
// Salah password menghasilkan apperror.ErrNotFound (401); setelah maxLinkAttempts kali salah, link token dibakar.
func (s *Service) LinkGoogle(ctx context.Context, linkToken, password string) (string, error) {
	userID, googleID, err := s.issuer.VerifyLink(linkToken)
	if err != nil {
		return "", apperror.ErrInvalidLinkToken
	}
	key := linkKey(linkToken)
	if s.links.exhausted(key, s.now()) {
		return "", apperror.ErrInvalidLinkToken
	}

	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, apperror.ErrNotFound) {
		return "", apperror.ErrInvalidLinkToken
	}
	if err != nil {
		return "", err
	}
	// Akun tanpa password tidak bisa membuktikan apa pun.
	if !user.PasswordHash.Valid {
		return "", apperror.ErrInvalidLinkToken
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(password)) != nil {
		s.links.fail(key, s.now())
		return "", apperror.ErrNotFound
	}

	// Sudah tertaut ke akun Google yang sama: idempoten, cukup terbitkan token.
	switch {
	case user.GoogleID.Valid && user.GoogleID.String == googleID:
		return s.issuer.Issue(user.ID)
	case user.GoogleID.Valid:
		return "", apperror.ErrDuplicate
	}
	if err := s.users.LinkGoogle(ctx, user.ID, googleID); err != nil {
		return "", err
	}
	// Link token sekali pakai: setelah berhasil, tidak bisa dipakai lagi.
	s.links.burn(key, s.now())
	return s.issuer.Issue(user.ID)
}

// usernameFor memilih username untuk akun Google baru: nama di akun Google, atau bagian depan email bila kosong.
// Karakter '<' dan '>' dibuang (sama dengan aturan no_html_tags), spasi dirapikan, dan panjang dibatasi 50 karakter.
func usernameFor(ident GoogleIdentity) string {
	if name := cleanName(ident.Name); name != "" {
		return name
	}
	if local, _, ok := strings.Cut(ident.Email, "@"); ok {
		if name := cleanName(local); name != "" {
			return name
		}
	}
	return "user"
}

// cleanName membuang '<' '>' dan karakter kontrol, merapikan spasi, dan memotong ke maxUsernameLen karakter.
func cleanName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '<' || r == '>' || r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > maxUsernameLen {
		s = string([]rune(s)[:maxUsernameLen])
		s = strings.TrimSpace(s)
	}
	return s
}

// linkKey: kunci penghitung percobaan, berupa hash link token (token mentah tidak disimpan di memori lebih lama
// dari yang perlu).
func linkKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// linkAttempts menghitung salah password per link token di memori. Link token hanya hidup 5 menit, jadi entri
// dibersihkan saat lewat batas itu. Cukup untuk satu instance (auth-service berjalan satu instance).
type linkAttempts struct {
	mu sync.Mutex
	// byKey: jumlah salah password dan kapan entri boleh dibuang.
	byKey map[string]*attemptEntry
}

type attemptEntry struct {
	fails   int
	burned  bool
	expires time.Time
}

func newLinkAttempts() *linkAttempts { return &linkAttempts{byKey: map[string]*attemptEntry{}} }

// prune membuang entri yang sudah lewat masa hidup link token. Dipanggil dengan mu terkunci.
func (l *linkAttempts) prune(now time.Time) {
	for k, e := range l.byKey {
		if now.After(e.expires) {
			delete(l.byKey, k)
		}
	}
}

func (l *linkAttempts) entry(key string, now time.Time) *attemptEntry {
	e, ok := l.byKey[key]
	if !ok {
		e = &attemptEntry{expires: now.Add(linkTokenTTL + time.Minute)}
		l.byKey[key] = e
	}
	return e
}

// exhausted: true bila token sudah dibakar atau terlalu sering salah password.
func (l *linkAttempts) exhausted(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	e, ok := l.byKey[key]
	return ok && (e.burned || e.fails >= maxLinkAttempts)
}

// fail mencatat satu salah password.
func (l *linkAttempts) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entry(key, now).fails++
}

// burn menandai token terpakai (sekali pakai).
func (l *linkAttempts) burn(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entry(key, now).burned = true
}
