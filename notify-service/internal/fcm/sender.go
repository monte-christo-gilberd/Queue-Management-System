package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"qms/notify-service/internal/apperror"
	"qms/notify-service/internal/dto"
	"qms/notify-service/internal/model"
	"qms/notify-service/internal/pusher"
)

// ChannelID: kanal notifikasi Android yang dibuat aplikasi (frontend/src/lib/push-native-core.ts). Kedua nilai
// harus sama; kanal yang tidak dikenal membuat Android memakai kanal bawaan dengan prioritas lebih rendah.
const ChannelID = "queue-turn"

// defaultBaseURL: alamat FCM HTTP v1.
const defaultBaseURL = "https://fcm.googleapis.com"

// Sender: pengirim push ke perangkat Android lewat FCM HTTP v1. Padanan pusher.Sender untuk Web Push; keduanya
// mengembalikan galat yang sama (apperror) sehingga NotifyService tidak perlu tahu kanal mana yang dipakai.
type Sender struct {
	// projectID: project Firebase tujuan (bagian URL messages:send).
	projectID string
	// baseURL: alamat FCM; diganti server palsu di tes.
	baseURL string
	// client: klien HTTP dengan batas waktu 8 detik per permintaan, sama dengan Web Push.
	client *http.Client
	// tokens: penyedia access token OAuth2 (di-cache).
	tokens *tokenSource
	// retryBase: jeda awal retry (bawaan 1 detik; dipendekkan di tes).
	retryBase time.Duration
}

// Option mengubah Sender saat dibuat (dipakai tes).
type Option func(*Sender)

// WithBaseURL mengganti alamat FCM (mis. server palsu di tes).
func WithBaseURL(u string) Option { return func(s *Sender) { s.baseURL = strings.TrimRight(u, "/") } }

// WithRetryBase mengganti jeda awal retry.
func WithRetryBase(d time.Duration) Option { return func(s *Sender) { s.retryBase = d } }

// WithClock mengganti sumber waktu cache access token (untuk tes kedaluwarsa).
func WithClock(now func() time.Time) Option {
	return func(s *Sender) { s.tokens.now = now }
}

// New membuat Sender untuk projectID dengan kredensial service account creds. projectID kosong diisi dari
// project_id kredensial. Galat bila project tidak diketahui atau kunci privat tidak sah.
func New(projectID string, creds Credentials, opts ...Option) (*Sender, error) {
	// FCM_PROJECT_ID kosong: pakai project dari berkas kredensial.
	if projectID == "" {
		projectID = creds.ProjectID
	}
	// Tanpa project, URL pengiriman tidak bisa disusun.
	if projectID == "" {
		return nil, errors.New("fcm: project id is required")
	}
	// Satu klien HTTP untuk token OAuth dan pengiriman.
	client := &http.Client{Timeout: 8 * time.Second}
	ts, err := newTokenSource(creds, client, time.Now)
	if err != nil {
		return nil, err
	}
	s := &Sender{projectID: projectID, baseURL: defaultBaseURL, client: client, tokens: ts, retryBase: time.Second}
	// Terapkan opsi (hanya dari tes).
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// Send mengirim satu push ke token FCM; kegagalan sementara dicoba ulang 3 kali (1s, 2s), sama seperti Web Push.
func (s *Sender) Send(ctx context.Context, sub model.Subscription, msg dto.Message) error {
	// Token kosong tidak mungkin terkirim; tolak tanpa memanggil FCM.
	if sub.Token == "" {
		return errors.New("fcm: empty device token")
	}
	// Susun body JSON sekali, dipakai ulang di setiap percobaan.
	body, err := json.Marshal(payload(sub.Token, msg))
	if err != nil {
		return err
	}
	// Pakai mekanisme retry yang sama dengan Web Push: hanya ErrTemporaryPush yang diulang.
	return pusher.Retry(ctx, 3, s.retryBase, func() error { return s.once(ctx, body) })
}

// payload menyusun pesan FCM v1:
//   - notification: judul dan isi, ditampilkan sistem Android walau aplikasi ditutup;
//   - data.url: path yang dibuka saat notifikasi diketuk (dibaca hooks/use-push-listeners.native.ts);
//   - android.priority HIGH: diantar segera walau perangkat dalam mode Doze;
//   - android.ttl 60s: pemberitahuan giliran cepat basi, jadi tidak perlu diantar bila perangkat lama offline;
//   - android.notification.channel_id: kanal `queue-turn` buatan aplikasi.
func payload(token string, msg dto.Message) map[string]any {
	return map[string]any{
		"message": map[string]any{
			"token":        token,
			"notification": map[string]string{"title": msg.Title, "body": msg.Body},
			// Nilai di "data" wajib string menurut FCM.
			"data": map[string]string{"url": msg.URL},
			"android": map[string]any{
				"priority":     "HIGH",
				"ttl":          "60s",
				"notification": map[string]string{"channel_id": ChannelID},
			},
		},
	}
}

// fcmError: bentuk galat FCM v1, yaitu google.rpc.Status dengan detail FcmError yang memuat errorCode.
type fcmError struct {
	Error struct {
		// Code: status HTTP.
		Code int `json:"code"`
		// Message: penjelasan galat dari Google.
		Message string `json:"message"`
		// Status: kode gRPC (mis. NOT_FOUND, INVALID_ARGUMENT).
		Status string `json:"status"`
		// Details: detail galat; yang bertipe FcmError membawa errorCode (mis. UNREGISTERED).
		Details []struct {
			Type      string `json:"@type"`
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error"`
}

// once mengirim satu kali tanpa retry, lalu menerjemahkan hasilnya ke ErrSubscriptionGone, ErrTemporaryPush, atau nil.
func (s *Sender) once(ctx context.Context, body []byte) error {
	// Ambil access token (dari cache, atau tukar baru bila hampir habis).
	tok, err := s.tokens.Token(ctx)
	if err != nil {
		return err
	}
	// POST ke endpoint messages:send milik project.
	url := fmt.Sprintf("%s/v1/projects/%s/messages:send", s.baseURL, s.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		// Konteks batal: kembalikan galat konteks, jangan dicoba ulang.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Galat jaringan: tandai sebagai sementara agar dicoba ulang.
		return fmt.Errorf("%w: %v", apperror.ErrTemporaryPush, err)
	}
	defer resp.Body.Close()
	// Baca body dengan batas 64 KiB; dibutuhkan untuk membaca errorCode.
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	// 200: pesan diterima FCM.
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	// Selain 200: petakan galatnya.
	return classify(s, resp.StatusCode, raw)
}

// classify memetakan respons galat FCM ke galat aplikasi:
//   - UNREGISTERED, SENDER_ID_MISMATCH, atau token tidak sah -> ErrSubscriptionGone (token dihapus dari push_devices);
//   - 401 -> access token dibuang, lalu ErrTemporaryPush (dicoba ulang dengan token baru);
//   - 429, 5xx, UNAVAILABLE, INTERNAL, QUOTA_EXCEEDED -> ErrTemporaryPush;
//   - lainnya -> galat biasa (dicatat, token tidak dihapus).
func classify(s *Sender, status int, raw []byte) error {
	// Body yang bukan JSON dibiarkan kosong; status HTTP tetap dipakai.
	var e fcmError
	_ = json.Unmarshal(raw, &e)
	// Ambil errorCode pertama dari detail FcmError.
	code := ""
	for _, d := range e.Error.Details {
		if d.ErrorCode != "" {
			code = d.ErrorCode
			break
		}
	}
	switch {
	// Perangkat mencabut token (aplikasi di-uninstall, data dihapus), atau token milik project Firebase lain.
	case code == "UNREGISTERED" || code == "SENDER_ID_MISMATCH":
		return apperror.ErrSubscriptionGone
	// INVALID_ARGUMENT dipakai untuk token tidak sah DAN payload rusak. Hanya dianggap token mati bila pesannya
	// menyebut registration token; selain itu galat biasa, supaya bug payload tidak menghapus semua token yang sah.
	case code == "INVALID_ARGUMENT" || (code == "" && e.Error.Status == "INVALID_ARGUMENT"):
		if strings.Contains(strings.ToLower(e.Error.Message), "registration token") {
			return apperror.ErrSubscriptionGone
		}
		return fmt.Errorf("fcm: invalid argument: %s", e.Error.Message)
	// Access token kedaluwarsa atau dicabut: buang dari cache supaya percobaan berikutnya memakai token baru.
	case status == http.StatusUnauthorized:
		s.tokens.Invalidate()
		return fmt.Errorf("%w: fcm status 401", apperror.ErrTemporaryPush)
	// FCM sibuk, kuota habis sementara, atau galat server: layak dicoba ulang.
	case status == http.StatusTooManyRequests || status >= 500 ||
		code == "UNAVAILABLE" || code == "INTERNAL" || code == "QUOTA_EXCEEDED":
		return fmt.Errorf("%w: fcm status %d %s", apperror.ErrTemporaryPush, status, code)
	}
	// Galat lain (mis. THIRD_PARTY_AUTH_ERROR, izin service account kurang): galat biasa, token dibiarkan.
	return fmt.Errorf("fcm: status %d %s: %s", status, code, e.Error.Message)
}
