// Tes paket fcm dan pusher.Dispatcher. Tidak ada request ke Google: satu server httptest berperan sebagai server
// token OAuth (/token) sekaligus FCM (/v1/projects/proj-1/messages:send), dengan kunci RSA yang dibuat saat tes.
package fcm_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"qms/notify-service/internal/apperror"
	"qms/notify-service/internal/dto"
	"qms/notify-service/internal/fcm"
	"qms/notify-service/internal/model"
	"qms/notify-service/internal/pusher"
)

// fakeGoogle: server palsu untuk endpoint token OAuth dan endpoint kirim FCM. Mencatat jumlah panggilan, body dan
// header Authorization terakhir, sehingga tes bisa memeriksa payload, cache token, dan jumlah retry.
type fakeGoogle struct {
	srv        *httptest.Server
	key        *rsa.PrivateKey
	tokenCalls atomic.Int32
	sendCalls  atomic.Int32

	mu       sync.Mutex
	lastBody map[string]any
	lastAuth string
	// replies: jawaban berurutan untuk setiap kiriman; jawaban terakhir dipakai ulang. Kosong = selalu 200.
	replies []reply
}

// reply: satu jawaban HTTP dari endpoint kirim FCM palsu.
type reply struct {
	status int
	body   string
}

// newFakeGoogle membuat kunci RSA service account dan menjalankan server palsu; ditutup otomatis di akhir tes.
func newFakeGoogle(t *testing.T, replies ...reply) *fakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGoogle{key: key, replies: replies}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve(t)))
	t.Cleanup(f.srv.Close)
	return f
}

// serve melayani dua endpoint: /token (cek grant_type dan JWT, lalu beri access token at-1, at-2, ...) dan
// messages:send (simpan body dan Authorization, lalu jawab sesuai replies).
func (f *fakeGoogle) serve(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			n := f.tokenCalls.Add(1)
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
				t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
			}
			f.verifyJWT(t, r.Form.Get("assertion"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"at-`+string(rune('0'+n))+`","expires_in":3600}`)
		case r.URL.Path == "/v1/projects/proj-1/messages:send":
			n := int(f.sendCalls.Add(1))
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.lastBody, f.lastAuth = body, r.Header.Get("Authorization")
			rep := reply{status: 200, body: `{"name":"projects/proj-1/messages/1"}`}
			if len(f.replies) > 0 {
				rep = f.replies[min(n, len(f.replies))-1]
			}
			f.mu.Unlock()
			w.WriteHeader(rep.status)
			_, _ = io.WriteString(w, rep.body)
		default:
			http.NotFound(w, r)
		}
	}
}

// verifyJWT memastikan assertion adalah JWT RS256 yang ditandatangani kunci service account, dengan iss, scope FCM,
// dan aud (token_uri) yang benar. Ini membuktikan signJWT menghasilkan JWT yang akan diterima Google.
func (f *fakeGoogle) verifyJWT(t *testing.T, jwt string) {
	t.Helper()
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("assertion is not a JWT: %q", jwt)
	}
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&f.key.PublicKey, crypto.SHA256, sum[:], sig); err != nil {
		t.Errorf("bad JWT signature: %v", err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	_ = json.Unmarshal(raw, &claims)
	if claims["iss"] != "sa@proj-1.iam.gserviceaccount.com" ||
		claims["scope"] != "https://www.googleapis.com/auth/firebase.messaging" ||
		claims["aud"] != f.srv.URL+"/token" {
		t.Errorf("claims = %v", claims)
	}
}

// credsJSON menyusun berkas JSON service account (bentuk yang sama dengan unduhan Firebase) dengan token_uri ke
// server palsu.
func (f *fakeGoogle) credsJSON(t *testing.T) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(f.key)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	b, _ := json.Marshal(map[string]string{
		"type":         "service_account",
		"project_id":   "proj-1",
		"client_email": "sa@proj-1.iam.gserviceaccount.com",
		"private_key":  string(pemKey),
		"token_uri":    f.srv.URL + "/token",
	})
	return b
}

// sender membuat fcm.Sender yang mengarah ke server palsu, dengan jeda retry 1 ms supaya tes cepat. Project id
// sengaja kosong untuk menguji pengisian dari project_id kredensial.
func (f *fakeGoogle) sender(t *testing.T, opts ...fcm.Option) *fcm.Sender {
	t.Helper()
	creds, err := fcm.ParseCredentials(f.credsJSON(t))
	if err != nil {
		t.Fatal(err)
	}
	opts = append([]fcm.Option{fcm.WithBaseURL(f.srv.URL), fcm.WithRetryBase(time.Millisecond)}, opts...)
	s, err := fcm.New("", creds, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

var (
	testMsg = dto.Message{Title: "Poli", Body: "Giliran Anda!", URL: "/ticket/12"}
	testSub = model.Subscription{Kind: model.KindFCM, Token: "device-tok"}
)

// Payload FCM v1: token, notification, data.url, prioritas HIGH, TTL 60s, dan kanal queue-turn; access token dikirim
// sebagai Bearer.
func TestSendBuildsV1Payload(t *testing.T) {
	f := newFakeGoogle(t)
	if err := f.sender(t).Send(context.Background(), testSub, testMsg); err != nil {
		t.Fatal(err)
	}
	if f.lastAuth != "Bearer at-1" {
		t.Errorf("Authorization = %q", f.lastAuth)
	}
	m := f.lastBody["message"].(map[string]any)
	n := m["notification"].(map[string]any)
	a := m["android"].(map[string]any)
	if m["token"] != "device-tok" || n["title"] != "Poli" || n["body"] != "Giliran Anda!" ||
		m["data"].(map[string]any)["url"] != "/ticket/12" ||
		a["priority"] != "HIGH" || a["ttl"] != "60s" ||
		a["notification"].(map[string]any)["channel_id"] != "queue-turn" {
		t.Fatalf("payload = %v", f.lastBody)
	}
}

// Access token di-cache: tiga kiriman hanya satu kali menukar token, dan token baru diambil saat sisa masa
// berlakunya kurang dari 1 menit.
func TestAccessTokenIsCachedUntilNearExpiry(t *testing.T) {
	f := newFakeGoogle(t)
	now := time.Unix(1_700_000_000, 0)
	s := f.sender(t, fcm.WithClock(func() time.Time { return now }))
	ctx := context.Background()
	for range 3 {
		if err := s.Send(ctx, testSub, testMsg); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.tokenCalls.Load(); got != 1 {
		t.Fatalf("token fetched %d times, want 1 (cached)", got)
	}
	// 59.5 minutes later the token has less than a minute left: fetch a new one.
	now = now.Add(59*time.Minute + 30*time.Second)
	if err := s.Send(ctx, testSub, testMsg); err != nil {
		t.Fatal(err)
	}
	if got := f.tokenCalls.Load(); got != 2 {
		t.Fatalf("token fetched %d times, want 2 (refreshed)", got)
	}
}

// fcmErr menyusun jawaban galat berbentuk FCM v1 (google.rpc.Status dengan detail FcmError bila code diisi).
func fcmErr(status int, rpcStatus, code, msg string) reply {
	details := ""
	if code != "" {
		details = `,"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"` + code + `"}]`
	}
	return reply{status: status, body: `{"error":{"code":` + itoa(status) + `,"message":"` + msg + `","status":"` + rpcStatus + `"` + details + `}}`}
}

// itoa mengubah angka ke teks JSON.
func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// Pemetaan galat FCM: token mati menjadi ErrSubscriptionGone (tanpa retry), galat sementara menjadi ErrTemporaryPush
// (dicoba 3 kali), dan INVALID_ARGUMENT karena payload tidak menghapus token.
func TestErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rep   reply
		gone  bool
		temp  bool
		calls int32
	}{
		{"unregistered", fcmErr(404, "NOT_FOUND", "UNREGISTERED", "Requested entity was not found."), true, false, 1},
		{"sender mismatch", fcmErr(403, "PERMISSION_DENIED", "SENDER_ID_MISMATCH", "SenderId mismatch"), true, false, 1},
		{"invalid token", fcmErr(400, "INVALID_ARGUMENT", "INVALID_ARGUMENT", "The registration token is not a valid FCM registration token"), true, false, 1},
		{"invalid payload keeps token", fcmErr(400, "INVALID_ARGUMENT", "INVALID_ARGUMENT", "Invalid value at 'message.android.ttl'"), false, false, 1},
		{"unavailable retried", fcmErr(503, "UNAVAILABLE", "UNAVAILABLE", "busy"), false, true, 3},
		{"internal retried", fcmErr(500, "INTERNAL", "INTERNAL", "oops"), false, true, 3},
		{"quota retried", fcmErr(429, "RESOURCE_EXHAUSTED", "QUOTA_EXCEEDED", "slow down"), false, true, 3},
		{"other error", fcmErr(403, "PERMISSION_DENIED", "THIRD_PARTY_AUTH_ERROR", "nope"), false, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGoogle(t, tc.rep)
			err := f.sender(t).Send(context.Background(), testSub, testMsg)
			if err == nil {
				t.Fatal("want error")
			}
			if got := errors.Is(err, apperror.ErrSubscriptionGone); got != tc.gone {
				t.Errorf("gone = %v, want %v (err %v)", got, tc.gone, err)
			}
			if got := errors.Is(err, apperror.ErrTemporaryPush); got != tc.temp {
				t.Errorf("temporary = %v, want %v (err %v)", got, tc.temp, err)
			}
			if got := f.sendCalls.Load(); got != tc.calls {
				t.Errorf("send attempts = %d, want %d", got, tc.calls)
			}
		})
	}
}

// 401 dari FCM membuang access token tersimpan; percobaan ulang memakai token baru (at-2) dan berhasil.
func TestUnauthorizedRefreshesAccessTokenAndRetries(t *testing.T) {
	f := newFakeGoogle(t, fcmErr(401, "UNAUTHENTICATED", "", "expired"), reply{status: 200, body: `{}`})
	if err := f.sender(t).Send(context.Background(), testSub, testMsg); err != nil {
		t.Fatal(err)
	}
	if f.tokenCalls.Load() != 2 || f.lastAuth != "Bearer at-2" {
		t.Fatalf("tokenCalls=%d lastAuth=%q, want refreshed token", f.tokenCalls.Load(), f.lastAuth)
	}
}

// Langganan FCM tanpa token ditolak tanpa request apa pun ke Google.
func TestEmptyTokenIsRejectedWithoutCalls(t *testing.T) {
	f := newFakeGoogle(t)
	if err := f.sender(t).Send(context.Background(), model.Subscription{Kind: model.KindFCM}, testMsg); err == nil {
		t.Fatal("want error")
	}
	if f.sendCalls.Load() != 0 || f.tokenCalls.Load() != 0 {
		t.Fatal("no request expected")
	}
}

// Berkas kredensial wajib punya client_email dan private_key; token_uri kosong diisi alamat Google; kunci privat
// yang bukan PEM ditolak saat fcm.New (saat start, bukan saat kirim pertama).
func TestParseCredentialsRequiresKeyAndEmail(t *testing.T) {
	for _, in := range []string{`nope`, `{}`, `{"client_email":"a"}`, `{"private_key":"k"}`} {
		if _, err := fcm.ParseCredentials([]byte(in)); err == nil {
			t.Errorf("ParseCredentials(%s): want error", in)
		}
	}
	c, err := fcm.ParseCredentials([]byte(`{"client_email":"a","private_key":"k"}`))
	if err != nil || c.TokenURI != "https://oauth2.googleapis.com/token" {
		t.Fatalf("c=%+v err=%v, want default token_uri", c, err)
	}
	if _, err := fcm.New("p", c); err == nil {
		t.Fatal("non-PEM private key: want error")
	}
}

// recordSender: pengirim palsu yang mencatat langganan yang diterimanya.
type recordSender struct{ got []model.Subscription }

func (r *recordSender) Send(_ context.Context, s model.Subscription, _ dto.Message) error {
	r.got = append(r.got, s)
	return nil
}

// Dispatcher: Kind kosong dan webpush ke pengirim Web Push, Kind fcm ke pengirim FCM.
func TestDispatcherRoutesByKind(t *testing.T) {
	web, android := &recordSender{}, &recordSender{}
	d := pusher.NewDispatcher(web, android)
	ctx := context.Background()
	_ = d.Send(ctx, model.Subscription{Endpoint: "https://push/a"}, testMsg)
	_ = d.Send(ctx, model.Subscription{Kind: model.KindWebPush, Endpoint: "https://push/b"}, testMsg)
	_ = d.Send(ctx, testSub, testMsg)
	if len(web.got) != 2 || len(android.got) != 1 || android.got[0].Token != "device-tok" {
		t.Fatalf("web=%v android=%v", web.got, android.got)
	}
}

// Tanpa pengirim FCM (kredensial kosong): langganan FCM ditolak ErrChannelDisabled, Web Push tetap terkirim.
func TestDispatcherWithoutFCMKeepsWebPush(t *testing.T) {
	web := &recordSender{}
	d := pusher.NewDispatcher(web, nil)
	if err := d.Send(context.Background(), testSub, testMsg); !errors.Is(err, pusher.ErrChannelDisabled) {
		t.Fatalf("err = %v, want ErrChannelDisabled", err)
	}
	if err := d.Send(context.Background(), model.Subscription{Endpoint: "https://push/a"}, testMsg); err != nil || len(web.got) != 1 {
		t.Fatalf("web push must still work: err=%v got=%v", err, web.got)
	}
}
