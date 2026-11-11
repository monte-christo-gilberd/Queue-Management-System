package service

import (
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"qms/qr-service/internal/apperror"
	"qms/qr-service/internal/model"
	"strings"
	"time"
)

const (
	// maxPayloadBytes: batas ukuran payload JSON yang sudah di-decode (512 byte).
	maxPayloadBytes = 512
	// minCustomTTL: batas bawah TTL yang diminta klien untuk QR dinamis (30 detik).
	minCustomTTL = 30 * time.Second
	// maxCustomTTL: batas atas TTL yang diminta klien untuk QR dinamis (10 menit).
	maxCustomTTL = 10 * time.Minute
)

// TokenService membuat dan memverifikasi token join bertanda tangan HMAC-SHA256.
// Token tidak menyimpan data rahasia; tanda tangan mencegah pemalsuan.
type TokenService struct {
	// secret: kunci HMAC untuk menandatangani dan memverifikasi token.
	secret []byte
	// dynamicTTL: masa berlaku bawaan token dinamis bila klien tidak mengaturnya.
	dynamicTTL time.Duration
	// now: sumber waktu; bisa diganti di tes.
	now func() time.Time
}

// NewTokenService membuat TokenService. Secret minimal 32 byte agar tidak mudah ditebak.
func NewTokenService(
	secret string,
	dynamicTTL time.Duration,
) (*TokenService, error) {
	// secret kurang dari 32 byte terlalu mudah ditebak: tolak
	if len(secret) < 32 {
		return nil, fmt.Errorf("join link secret must be at least 32 bytes")
	}
	// secret cukup panjang: buat service dengan time.Now sebagai sumber waktu
	return &TokenService{
		secret:     []byte(secret),
		dynamicTTL: dynamicTTL,
		now:        time.Now,
	}, nil
}

// IssueToken memeriksa aturan input (queue_id, jenis, TTL) lalu membuat token join baru.
// Token statis mengabaikan TTL; token dinamis memakai TTL klien (30 detik sampai 10 menit) atau TTL bawaan.
func (s *TokenService) IssueToken(
	queueID int64,
	typ string,
	ttlSeconds int,
) (string, model.Claims, error) {
	// queue_id tidak positif: tolak
	if queueID <= 0 {
		return "", model.Claims{}, apperror.ErrInvalidQueueID
	}

	// ubah teks jenis token menjadi tipe model.Type
	tokenType := model.Type(typ)
	// jenis token tidak dikenal (bukan static atau dynamic): tolak
	if !tokenType.Valid() {
		return "", model.Claims{}, apperror.ErrInvalidTokenType
	}

	// mulai dari TTL bawaan layanan
	ttl := s.dynamicTTL
	// token dinamis dan klien mengirim TTL sendiri: pakai TTL klien setelah memeriksa rentangnya
	if tokenType == model.TypeDynamic && ttlSeconds > 0 {
		// ubah detik dari klien menjadi durasi
		ttl = time.Duration(ttlSeconds) * time.Second
		// TTL di luar rentang yang diizinkan: tolak
		if ttl < minCustomTTL || ttl > maxCustomTTL {
			return "", model.Claims{}, apperror.ErrTTLOutOfRange
		}
	}

	// buat dan tandatangani token
	return s.Issue(queueID, tokenType, ttl)
}

// Issue membuat token untuk satu antrian.
// Token statis tidak punya exp; token dinamis memakai ttl (default model.DefaultDynamicTTL).
func (s *TokenService) Issue(
	queueID int64,
	typ model.Type,
	ttl time.Duration,
) (string, model.Claims, error) {
	// id antrian tidak positif: tolak sebagai payload tidak valid
	if queueID <= 0 {
		return "", model.Claims{}, apperror.ErrInvalidPayload
	}
	// jenis tidak dikenal: tolak sebagai payload tidak valid
	if !typ.Valid() {
		return "", model.Claims{}, apperror.ErrInvalidPayload
	}

	// buat id unik acak untuk token ini
	jti, err := randomID()
	// gagal membuat id acak: bungkus galatnya dan kembalikan
	if err != nil {
		return "", model.Claims{}, fmt.Errorf("generate jti: %w", err)
	}

	// susun klaim dasar: antrian, jenis, dan id unik
	claims := model.Claims{
		QueueID: queueID,
		Type:    typ,
		JTI:     jti,
	}
	// token dinamis: hitung waktu kedaluwarsa dari sekarang ditambah TTL
	if typ == model.TypeDynamic {
		// TTL tidak diisi (nol atau negatif): pakai bawaan
		if ttl <= 0 {
			ttl = model.DefaultDynamicTTL
		}
		// simpan waktu kedaluwarsa dalam detik Unix
		claims.ExpiresAt = s.now().Add(ttl).Unix()
	}

	// ubah klaim ke JSON sebagai payload token
	payload, err := json.Marshal(claims)
	// gagal membuat JSON: bungkus galatnya dan kembalikan
	if err != nil {
		return "", model.Claims{}, fmt.Errorf("marshal claims: %w", err)
	}
	// payload terlalu besar: tolak agar verifikasi nanti tetap cepat
	if len(payload) > maxPayloadBytes {
		return "", model.Claims{}, apperror.ErrInvalidPayload
	}

	// kodekan payload ke base64url tanpa padding, lalu gabungkan dengan versi
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := model.Version + "." + payloadB64
	// tandatangani bagian versi.payload, lalu tambahkan tanda tangan sebagai bagian ketiga
	token := signingInput + "." + s.sign(signingInput)

	// kembalikan token beserta klaimnya
	return token, claims, nil
}

// VerifyToken memeriksa format, tanda tangan, dan masa berlaku token.
func (s *TokenService) VerifyToken(
	token string,
) (model.Claims, error) {
	// token kosong atau terlalu panjang: tolak sebagai format salah
	if token == "" || len(token) > model.MaxTokenLength {
		return model.Claims{}, apperror.ErrTokenMalformed
	}

	// pecah token menjadi tiga bagian: versi, payload, dan tanda tangan
	parts := strings.Split(token, ".")
	// jumlah bagian bukan tiga: format salah
	if len(parts) != 3 {
		return model.Claims{}, apperror.ErrTokenMalformed
	}
	// versi tidak dikenal: tolak sebagai versi tidak didukung
	if parts[0] != model.Version {
		return model.Claims{}, apperror.ErrTokenUnsupportedFormat
	}

	// decode payload dari base64url; gagal decode, payload kosong, atau terlalu besar: tolak
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(payload) == 0 || len(payload) > maxPayloadBytes {
		return model.Claims{}, apperror.ErrTokenMalformed
	}

	// hitung ulang tanda tangan dari versi.payload, lalu bandingkan dalam waktu tetap
	expected := s.sign(parts[0] + "." + parts[1])
	// tanda tangan tidak cocok: token dipalsukan atau diubah
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return model.Claims{}, apperror.ErrTokenInvalidSignature
	}

	// tanda tangan benar: baca klaim dari payload JSON
	var claims model.Claims
	// JSON tidak bisa dibaca: format salah
	if err := json.Unmarshal(payload, &claims); err != nil {
		return model.Claims{}, apperror.ErrTokenMalformed
	}
	// id antrian tidak positif, jenis tidak dikenal, atau JTI kosong: isi payload tidak valid
	if claims.QueueID <= 0 || !claims.Type.Valid() || claims.JTI == "" {
		return model.Claims{}, apperror.ErrInvalidPayload
	}
	// token dinamis tanpa waktu kedaluwarsa: tidak boleh ada, jadi tolak
	if claims.Type == model.TypeDynamic && claims.ExpiresAt == 0 {
		return model.Claims{}, apperror.ErrInvalidPayload
	}
	// waktu sekarang sudah sampai atau melewati kedaluwarsa: token tidak berlaku lagi
	if claims.ExpiresAt > 0 && s.now().Unix() >= claims.ExpiresAt {
		return model.Claims{}, apperror.ErrTokenExpired
	}

	// semua pemeriksaan lolos: kembalikan klaim
	return claims, nil
}
