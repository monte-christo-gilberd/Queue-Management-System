package google

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/model"
)

const (
	// DefaultCertsURL: kunci publik (JWKS) penanda ID token Google.
	DefaultCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

	// leeway: toleransi selisih jam server terhadap Google untuk klaim iat dan exp.
	leeway = 60 * time.Second
	// defaultKeyTTL dipakai bila respons Google tidak memuat Cache-Control max-age.
	defaultKeyTTL = time.Hour
	// minRefreshInterval: batas paling cepat mengambil ulang kunci karena `kid` tidak dikenal, supaya token
	// dengan kid acak tidak membuat server membanjiri Google.
	minRefreshInterval = time.Minute
	// maxKeysBody membatasi isi respons JWKS yang dibaca.
	maxKeysBody = 1 << 20
)

// issuers: nilai klaim `iss` yang sah untuk ID token Google.
var issuers = map[string]bool{"https://accounts.google.com": true, "accounts.google.com": true}

// Verifier memeriksa ID token Google. Aman dipakai dari banyak goroutine.
type Verifier struct {
	// certsURL: alamat JWKS Google (bisa diganti di tes).
	certsURL string
	// audiences: client ID yang sah (web, Android, iOS). `aud` token harus salah satunya.
	audiences map[string]bool
	// client: klien HTTP dengan batas waktu untuk mengambil JWKS.
	client *http.Client
	// now: sumber waktu; bisa diganti di tes.
	now func() time.Time

	// mu: melindungi keys, expires, dan lastFetch dari akses bersamaan antargoroutine.
	mu sync.Mutex
	// keys: kunci publik per `kid`.
	keys map[string]*rsa.PublicKey
	// expires: kapan keys harus diambil ulang.
	expires time.Time
	// lastFetch: kapan terakhir mengambil JWKS (untuk throttle kid tak dikenal).
	lastFetch time.Time
}

// NewVerifier membuat Verifier untuk daftar client ID. certsURL kosong = DefaultCertsURL.
func NewVerifier(certsURL string, clientIDs []string) *Verifier {
	// certsURL kosong: pakai alamat resmi Google.
	if certsURL == "" {
		certsURL = DefaultCertsURL
	}
	// Daftar client ID dijadikan set agar pencocokan audience cepat.
	aud := make(map[string]bool, len(clientIDs))
	// Buang spasi dan lewati entri kosong supaya client ID kosong tidak pernah dianggap sah.
	for _, id := range clientIDs {
		if id = strings.TrimSpace(id); id != "" {
			aud[id] = true
		}
	}
	// Cache kunci mulai kosong. Klien HTTP berbatas 5 detik agar login tidak menggantung bila Google lambat.
	return &Verifier{
		certsURL:  certsURL,
		audiences: aud,
		client:    &http.Client{Timeout: 5 * time.Second},
		now:       time.Now,
		keys:      map[string]*rsa.PublicKey{},
	}
}

// WithClock mengganti sumber waktu. Hanya untuk tes (kedaluwarsa dan rotasi kunci tanpa menunggu).
func (v *Verifier) WithClock(now func() time.Time) *Verifier {
	// Ganti sumber waktu bawaan (time.Now) dengan jam palsu dari tes.
	v.now = now
	// Kembalikan Verifier yang sama agar pemanggilan bisa dirangkai.
	return v
}

// Verify memeriksa ID token dan mengembalikan identitasnya. Galat:
//   - apperror.ErrGoogleInvalidToken: tanda tangan, issuer, audience, atau masa berlaku tidak sah;
//   - apperror.ErrGoogleEmailNotVerified: Google tidak menjamin email itu milik pemegang akun;
//   - apperror.ErrGoogleUnavailable: kunci Google tidak bisa diambil.
func (v *Verifier) Verify(ctx context.Context, idToken string) (model.GoogleIdentity, error) {
	// Tanpa daftar client ID, tidak ada audience yang sah: semua token ditolak.
	if len(v.audiences) == 0 {
		// Tidak ada audience yang sah: semua token ditolak.
		return model.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}

	// Klaim exp/iat diperiksa sendiri di bawah dengan toleransi jam (pustaka tidak punya leeway).
	parser := &jwt.Parser{ValidMethods: []string{"RS256"}, SkipClaimsValidation: true}
	// Wadah klaim; baru diisi dan dipercaya setelah tanda tangan lolos.
	claims := jwt.MapClaims{}
	// Parse token dan periksa tanda tangan RS256 dengan kunci publik yang dipilih callback berdasarkan kid.
	// ValidMethods mencegah serangan pergantian algoritma (none atau HS256).
	_, err := parser.ParseWithClaims(idToken, claims, func(t *jwt.Token) (interface{}, error) {
		// Ambil kid dari header token untuk memilih kunci penandatangan.
		kid, _ := t.Header["kid"].(string)
		// Tanpa kid, kunci tidak bisa dipilih: tolak.
		if kid == "" {
			return nil, errors.New("missing kid")
		}
		// Cari kunci publik untuk kid, dari cache atau dengan mengambil JWKS Google.
		return v.key(ctx, kid)
	})
	// Parse gagal: tanda tangan salah, format rusak, algoritma tidak diizinkan, atau kunci tidak ada.
	if err != nil {
		// Kunci tidak bisa diambil: bukan salah token, jadi dibedakan dari token tidak sah.
		var ve *jwt.ValidationError
		// Penyebabnya gagal mengambil kunci Google, bukan token yang buruk.
		if errors.As(err, &ve) && errors.Is(ve.Inner, apperror.ErrGoogleUnavailable) {
			// Dikembalikan sebagai ErrGoogleUnavailable agar klien tahu boleh mencoba lagi nanti.
			return model.GoogleIdentity{}, apperror.ErrGoogleUnavailable
		}
		// Selain itu token dianggap tidak sah.
		return model.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}

	// Waktu dari sumber yang bisa disuntik, supaya tes bisa mengatur jam.
	now := v.now()
	// exp wajib dan harus di masa depan (dengan toleransi).
	exp, ok := numeric(claims["exp"])
	// Tolak bila exp hilang atau sudah lewat melebihi toleransi.
	if !ok || now.After(time.Unix(exp, 0).Add(leeway)) {
		// Token kedaluwarsa atau tanpa exp.
		return model.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	// iat tidak boleh di masa depan (dengan toleransi).
	if iat, ok := numeric(claims["iat"]); ok && time.Unix(iat, 0).After(now.Add(leeway)) {
		// Token berasal dari masa depan.
		return model.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	// Issuer harus Google.
	if iss, _ := claims["iss"].(string); !issuers[iss] {
		// Token bukan diterbitkan Google.
		return model.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	// Audience harus salah satu client ID kita. Token untuk aplikasi lain ditolak.
	if !v.audienceOK(claims["aud"]) {
		// Token dibuat untuk aplikasi lain.
		return model.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	// sub: id akun Google yang stabil.
	sub, _ := claims["sub"].(string)
	// email: alamat akun Google.
	email, _ := claims["email"].(string)
	// Tanpa sub atau email, akun tidak bisa dikenali: tolak.
	if sub == "" || email == "" {
		// Klaim wajib tidak lengkap.
		return model.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	// Email yang tidak diverifikasi Google tidak boleh dipakai untuk membuat atau mencocokkan akun.
	if !truthy(claims["email_verified"]) {
		// Dibedakan dari token tidak sah supaya pesan ke pengguna bisa lebih jelas.
		return model.GoogleIdentity{}, apperror.ErrGoogleEmailNotVerified
	}
	// Nama boleh kosong, jadi tidak divalidasi.
	name, _ := claims["name"].(string)
	// Semua pemeriksaan lolos: kembalikan identitas yang boleh dipercaya.
	return model.GoogleIdentity{Subject: sub, Email: email, Name: name}, nil
}

// audienceOK: `aud` berupa string, atau daftar string; cukup satu yang cocok dengan client ID kita.
func (v *Verifier) audienceOK(aud interface{}) bool {
	// Klaim aud bisa berupa satu string atau array string.
	switch a := aud.(type) {
	// Satu audience: cocokkan langsung.
	case string:
		// Benar bila audience itu salah satu client ID kita.
		return v.audiences[a]
	// Banyak audience: satu yang cocok sudah cukup.
	case []interface{}:
		// Periksa tiap entri; yang bukan string diabaikan.
		for _, item := range a {
			// Entri string yang cocok dengan client ID kita.
			if s, ok := item.(string); ok && v.audiences[s] {
				// Audience cocok.
				return true
			}
		}
	}
	// Tidak ada audience yang cocok: token ditolak.
	return false
}

// numeric membaca klaim waktu (JSON mengubah angka menjadi float64).
func numeric(v interface{}) (int64, bool) {
	// JSON decoder memberi float64 untuk angka; ok false bila klaim bukan angka.
	f, ok := v.(float64)
	// Ubah ke detik Unix.
	return int64(f), ok
}

// truthy membaca email_verified: Google mengirim boolean, tetapi beberapa token memakai string "true".
func truthy(v interface{}) bool {
	// Terima boolean atau string.
	switch b := v.(type) {
	// Bentuk boolean standar.
	case bool:
		// Pakai nilainya apa adanya.
		return b
	// Bentuk string dari penerbit yang tidak baku.
	case string:
		// Hanya string "true" yang dianggap terverifikasi.
		return b == "true"
	}
	// Tipe lain dianggap belum terverifikasi (aman secara bawaan).
	return false
}

// key mengembalikan kunci publik untuk kid, mengambil ulang JWKS bila perlu.
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Kunci mutex: cache dipakai banyak goroutine dan hanya satu yang boleh mengambil JWKS sekaligus.
	v.mu.Lock()
	// Lepas kunci saat fungsi selesai.
	defer v.mu.Unlock()
	// Waktu saat ini untuk menilai cache dan throttle.
	now := v.now()
	// Cache masih segar dan kid dikenal: pakai langsung tanpa panggilan jaringan.
	if k, ok := v.keys[kid]; ok && now.Before(v.expires) {
		// Kunci dari cache.
		return k, nil
	}
	// Ambil ulang bila cache kedaluwarsa, atau kid tak dikenal dan sudah lewat batas minimal sejak pengambilan terakhir
	// (Google merotasi kunci; kid baru harus bisa dikenali tanpa menunggu TTL).
	expired := !now.Before(v.expires)
	// Ambil ulang hanya bila perlu; throttle mencegah token dengan kid acak membanjiri Google.
	if expired || now.Sub(v.lastFetch) >= minRefreshInterval {
		// Ambil JWKS baru dan ganti cache.
		if err := v.fetch(ctx); err != nil {
			// Cache lama yang kedaluwarsa tidak dipakai: lebih aman gagal daripada menerima kunci yang mungkin dicabut.
			return nil, fmt.Errorf("%w: %v", apperror.ErrGoogleUnavailable, err)
		}
	}
	// Cari kid setelah cache mungkin diperbarui.
	k, ok := v.keys[kid]
	// kid tetap tidak ada: token ditandatangani kunci yang tidak kita kenal.
	if !ok {
		// Ditolak sebagai token tidak sah oleh Verify.
		return nil, errors.New("unknown kid")
	}
	// Kunci ditemukan setelah pengambilan ulang.
	return k, nil
}

// jwks: bentuk respons JWKS Google.
type jwks struct {
	// Keys: daftar kunci publik; tiap entri memuat kid, jenis kunci (kty), modulus (n), dan eksponen (e).
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

// fetch mengambil JWKS dan mengganti cache. Dipanggil dengan v.mu terkunci.
func (v *Verifier) fetch(ctx context.Context) error {
	// Catat waktu sebelum request supaya kegagalan pun ikut men-throttle percobaan berikutnya.
	v.lastFetch = v.now()
	// Buat request GET ke JWKS; ikut ctx agar batal bersama request login.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return err
	}
	// Kirim request; dibatasi timeout klien HTTP.
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	// Tutup body agar koneksi bisa dipakai ulang.
	defer resp.Body.Close()
	// Selain 200 berarti Google bermasalah: isinya tidak dipercaya.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	// Wadah hasil decode JWKS.
	var set jwks
	// Decode JSON; isi dibatasi maxKeysBody agar respons raksasa tidak menghabiskan memori.
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxKeysBody)).Decode(&set); err != nil {
		return err
	}
	// Peta kunci baru; cache lama baru diganti setelah respons selesai dibaca.
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	// Ubah tiap entri JWKS menjadi kunci publik RSA.
	for _, k := range set.Keys {
		// Lewati entri non-RSA atau tanpa kid; ID token Google memakai RS256.
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		// Bentuk kunci dari modulus dan eksponen.
		pub, err := rsaKey(k.N, k.E)
		if err != nil {
			continue
		}
		// Simpan per kid untuk pencarian cepat.
		keys[k.Kid] = pub
	}
	// Tanpa kunci yang bisa dipakai, cache lama dibiarkan dan galat dikembalikan.
	if len(keys) == 0 {
		return errors.New("no usable keys")
	}
	// Ganti seluruh cache sekaligus.
	v.keys = keys
	// Masa berlaku cache mengikuti max-age dari Google (dibatasi di maxAge).
	v.expires = v.now().Add(maxAge(resp.Header.Get("Cache-Control")))
	// Cache berhasil diperbarui.
	return nil
}

// rsaKey membentuk kunci publik RSA dari modulus (n) dan eksponen (e) base64url.
func rsaKey(n, e string) (*rsa.PublicKey, error) {
	// Decode modulus (base64url tanpa padding, sesuai format JWK).
	nb, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, err
	}
	// Decode eksponen publik.
	eb, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, err
	}
	// Eksponen sebagai bilangan besar.
	exp := new(big.Int).SetBytes(eb)
	// Tolak eksponen yang tidak muat di int atau terlalu kecil (di bawah 3 tidak aman untuk RSA).
	if !exp.IsInt64() || exp.Int64() < 3 {
		return nil, errors.New("bad exponent")
	}
	// Rakit kunci publik: N modulus, E eksponen.
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(exp.Int64())}, nil
}

// maxAge membaca max-age dari Cache-Control; tidak ada atau tidak valid = defaultKeyTTL. Dibatasi 1 menit sampai 24 jam.
func maxAge(cacheControl string) time.Duration {
	// Cache-Control berisi direktif dipisah koma; cari max-age.
	for _, part := range strings.Split(cacheControl, ",") {
		// Buang spasi di sekitar direktif.
		part = strings.TrimSpace(part)
		// Hanya direktif max-age yang dipakai.
		if v, ok := strings.CutPrefix(part, "max-age="); ok {
			// Nilai harus bilangan positif.
			if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
				// Ubah detik menjadi Duration.
				ttl := time.Duration(secs) * time.Second
				// Batasi antara 1 menit (hindari pengambilan beruntun) dan 24 jam (kunci yang dicabut tidak bertahan lama).
				return min(max(ttl, time.Minute), 24*time.Hour)
			}
		}
	}
	// Tidak ada max-age yang valid: pakai TTL bawaan.
	return defaultKeyTTL
}
