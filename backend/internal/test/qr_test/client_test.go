package qrclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"qms/internal/apperror"
	"qms/internal/qrclient"
	"qms/internal/requestid"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testServiceToken = "service-token"

// newTestClient menjalankan httptest server dan mengembalikan client yang menunjuk
// ke server itu. Handler boleh membaca request untuk assertion.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*qrclient.Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return qrclient.New(server.URL, testServiceToken, 2*time.Second), server
}

func TestIssueJoinToken_SuccessSendsServiceTokenAndTTL(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotToken  string
		gotBody   map[string]interface{}
	)
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Service-Token")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"v1.abc.sig","type":"dynamic","expires_at":1700000000}`))
	})

	out, err := client.IssueJoinToken(context.Background(), 42, "dynamic", 120)
	require.NoError(t, err)
	assert.Equal(t, "v1.abc.sig", out.Token)
	assert.Equal(t, "dynamic", out.Type)
	require.NotNil(t, out.ExpiresAt)
	assert.Equal(t, int64(1700000000), *out.ExpiresAt)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/internal/v1/join-token", gotPath)
	assert.Equal(t, testServiceToken, gotToken)
	assert.Equal(t, float64(42), gotBody["queue_id"])
	assert.Equal(t, "dynamic", gotBody["type"])
	assert.Equal(t, float64(120), gotBody["ttl_seconds"])
}

func TestIssueJoinToken_OmitsTTLWhenZero(t *testing.T) {
	var gotBody map[string]interface{}
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = w.Write([]byte(`{"token":"v1.abc.sig","type":"static","expires_at":null}`))
	})

	out, err := client.IssueJoinToken(context.Background(), 7, "static", 0)
	require.NoError(t, err)
	assert.Nil(t, out.ExpiresAt)
	_, hasTTL := gotBody["ttl_seconds"]
	assert.False(t, hasTTL, "TTL 0 berarti pakai default qr-service, jangan dikirim")
}

func TestIssueJoinToken_ErrorMapping(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{"tipe QR tidak dikenal", http.StatusBadRequest, `{"error":{"code":"QR_TYPE_INVALID"}}`, apperror.ErrQRTypeInvalid},
		{"bad request lain", http.StatusBadRequest, `{"error":{"code":"BAD_REQUEST"}}`, apperror.ErrQRUnavailable},
		{"unauthorized service token", http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED"}}`, apperror.ErrQRUnavailable},
		{"server error", http.StatusInternalServerError, `{"error":{"code":"INTERNAL"}}`, apperror.ErrQRUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			_, err := client.IssueJoinToken(context.Background(), 1, "static", 0)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestIssueJoinToken_BadPayloadIsUnavailable(t *testing.T) {
	tests := map[string]string{
		"json rusak":   `{`,
		"token kosong": `{"token":"","type":"static"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			})
			_, err := client.IssueJoinToken(context.Background(), 1, "static", 0)
			assert.ErrorIs(t, err, apperror.ErrQRUnavailable)
		})
	}
}

func TestVerifyJoinToken_Success(t *testing.T) {
	var gotPath, gotToken string
	var gotBody map[string]string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Service-Token")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = w.Write([]byte(`{"queue_id":42,"type":"static","expires_at":null}`))
	})

	claims, err := client.VerifyJoinToken(context.Background(), "v1.abc.sig")
	require.NoError(t, err)
	assert.Equal(t, int64(42), claims.QueueID)
	assert.Equal(t, "static", claims.Type)
	assert.Nil(t, claims.ExpiresAt)
	assert.Equal(t, "/internal/v1/join-token/verify", gotPath)
	assert.Equal(t, testServiceToken, gotToken)
	assert.Equal(t, "v1.abc.sig", gotBody["token"])
}

func TestVerifyJoinToken_ErrorMapping(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr error
	}{
		{"kedaluwarsa", http.StatusGone, apperror.ErrQRExpired},
		{"bad request", http.StatusBadRequest, apperror.ErrQRInvalid},
		{"service token salah", http.StatusUnauthorized, apperror.ErrQRInvalid},
		{"tidak ditemukan", http.StatusNotFound, apperror.ErrQRInvalid},
		{"server error", http.StatusInternalServerError, apperror.ErrQRUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":{"code":"X"}}`))
			})
			_, err := client.VerifyJoinToken(context.Background(), "v1.abc.sig")
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestVerifyJoinToken_BadPayloadIsInvalid(t *testing.T) {
	tests := map[string]string{
		"queue_id nol":     `{"queue_id":0,"type":"static"}`,
		"queue_id negatif": `{"queue_id":-3,"type":"static"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			})
			_, err := client.VerifyJoinToken(context.Background(), "v1.abc.sig")
			assert.ErrorIs(t, err, apperror.ErrQRInvalid)
		})
	}
}

func TestVerifyJoinToken_MalformedJSONIsUnavailable(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{`))
	})
	_, err := client.VerifyJoinToken(context.Background(), "v1.abc.sig")
	assert.ErrorIs(t, err, apperror.ErrQRUnavailable)
}

func TestClient_NetworkFailuresAreUnavailable(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			_, _ = w.Write([]byte(`{"token":"x"}`))
		}))
		defer server.Close()
		client := qrclient.New(server.URL, testServiceToken, 10*time.Millisecond)

		_, err := client.IssueJoinToken(context.Background(), 1, "static", 0)
		assert.ErrorIs(t, err, apperror.ErrQRUnavailable)
		_, err = client.VerifyJoinToken(context.Background(), "v1.abc.sig")
		assert.ErrorIs(t, err, apperror.ErrQRUnavailable)
	})

	t.Run("koneksi ditolak", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := server.URL
		server.Close()

		client := qrclient.New(url, testServiceToken, time.Second)
		_, err := client.IssueJoinToken(context.Background(), 1, "static", 0)
		assert.ErrorIs(t, err, apperror.ErrQRUnavailable)
	})
}

func TestNew_DefaultTimeoutWhenNonPositive(t *testing.T) {
	// Timeout 0 harus jatuh ke default (10 detik), bukan berarti tanpa timeout.
	// Diuji lewat perilaku: server yang lambat 100 ms tetap berhasil.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"token":"v1.abc.sig","type":"static","expires_at":null}`))
	}))
	defer server.Close()

	client := qrclient.New(server.URL, testServiceToken, 0)
	out, err := client.IssueJoinToken(context.Background(), 1, "static", 0)
	require.NoError(t, err)
	assert.Equal(t, "v1.abc.sig", out.Token)
}

func TestNew_TrimsTrailingSlash(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"token":"v1.abc.sig","type":"static","expires_at":null}`))
	}))
	defer server.Close()

	client := qrclient.New(server.URL+"/", testServiceToken, time.Second)
	_, err := client.IssueJoinToken(context.Background(), 1, "static", 0)
	require.NoError(t, err)
	// Tanpa trim, path menjadi "//internal/v1/join-token".
	assert.Equal(t, "/internal/v1/join-token", gotPath)
}

// Request ID di context diteruskan ke qr-service lewat header X-Request-Id; tanpa ID di context, header tidak dikirim.
func TestClient_ForwardsRequestID(t *testing.T) {
	// header yang diterima server palsu qr-service
	var gotID string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get("X-Request-Id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"v1.abc.sig","type":"dynamic","expires_at":null}`))
	})

	// dengan ID di context: header X-Request-Id harus sampai ke qr-service
	_, err := client.IssueJoinToken(requestid.With(context.Background(), "req-xyz"), 1, "dynamic", 0)
	require.NoError(t, err)
	assert.Equal(t, "req-xyz", gotID)

	// tanpa ID di context: header tidak dikirim sama sekali
	_, err = client.IssueJoinToken(context.Background(), 1, "dynamic", 0)
	require.NoError(t, err)
	assert.Empty(t, gotID)
}

// Header traceparent diteruskan ke qr-service bila ada span aktif, sehingga trace backend dan qr-service menyatu.
func TestClient_PropagatesTraceparent(t *testing.T) {
	// propagator W3C dan provider sungguhan, seperti yang dipasang telemetry.Setup di produksi
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// header traceparent yang diterima server palsu qr-service
	var gotParent string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotParent = r.Header.Get("traceparent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"v1.abc.sig","type":"dynamic","expires_at":null}`))
	})

	// span induk meniru span server backend; context-nya dipakai untuk memanggil qr-service
	ctx, span := tp.Tracer("t").Start(context.Background(), "request")
	_, err := client.IssueJoinToken(ctx, 1, "dynamic", 0)
	span.End()
	require.NoError(t, err)
	// header ada dan memuat trace ID induk, sehingga qr-service ikut dalam trace yang sama
	require.NotEmpty(t, gotParent, "traceparent harus dikirim ke qr-service")
	assert.Contains(t, gotParent, span.SpanContext().TraceID().String())
}
