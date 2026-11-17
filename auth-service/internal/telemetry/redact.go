package telemetry

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// deniedKeys: atribut yang tidak boleh diekspor sama sekali. Berisi path atau query mentah (bisa memuat token layar
// publik, token join, atau token reset), alamat klien, dan header. Nama route (http.route) dan status tetap ada.
// Daftar mencakup nama lama (http.target) dan baru (url.path) karena versi instrumentasi bisa berganti konvensi.
var deniedKeys = map[string]bool{
	"url.full": true, "url.path": true, "url.query": true, "url.fragment": true,
	"http.target": true, "http.url": true, "http.user_agent": true, "user_agent.original": true,
	"client.address": true, "client.port": true, "http.client_ip": true,
	"net.sock.peer.addr": true, "net.sock.peer.port": true, "network.peer.address": true, "network.peer.port": true,
	"net.peer.name": true, "net.peer.port": true, "net.host.name": true, "net.host.port": true,
	"http.request.body": true, "http.response.body": true,
}

// deniedPrefixes: kelompok atribut yang dibuang, termasuk header dan nilai parameter query database.
// Awalan dipakai karena nama atributnya dinamis (mis. http.request.header.authorization).
var deniedPrefixes = []string{
	"http.request.header.", "http.response.header.", "http.request_content", "db.query.parameter.", "db.operation.parameter.",
}

// secretPatterns: lapis kedua. Nilai string yang masih memuat bentuk rahasia (Bearer, JWT, token join, token=...)
// diganti penanda. Sama dengan penyamaran di collector, supaya rahasia tertahan dua kali sebelum sampai Tempo.
var secretPatterns = []struct {
	// re: pola yang dicari di nilai atribut
	re *regexp.Regexp
	// repl: pengganti; ${1} mempertahankan nama (token/password/secret) dan hanya nilainya diganti
	repl string
}{
	// header Authorization: Bearer <token>
	{regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`), "Bearer ***"},
	// JWT: tiga bagian base64url dipisah titik, diawali eyJ
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`), "***jwt***"},
	// token join QR (bentuk warisan dari qr-service yang sudah dihapus; auth-service tidak membuatnya): v1.<payload>.<tanda tangan>
	{regexp.MustCompile(`\bv1\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`), "***join-token***"},
	// token=..., password=..., secret=... di query string atau teks bebas
	{regexp.MustCompile(`(?i)(token|password|secret)=[^&\s"]+`), "${1}=***"},
}

// denied melaporkan apakah atribut dengan kunci ini harus dibuang.
func denied(
	key string,
) bool {
	// cocok persis dengan daftar kunci terlarang
	if deniedKeys[key] {
		return true
	}
	// cocok dengan salah satu awalan kelompok terlarang
	for _, p := range deniedPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// scrub mengganti bentuk rahasia di s sesuai secretPatterns.
func scrub(
	s string,
) string {
	// jalankan semua pola berurutan; satu nilai bisa memuat lebih dari satu rahasia
	for _, p := range secretPatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return s
}

// filterAttrs membuang atribut terlarang dan menyamarkan nilai string; mengembalikan salinan baru
// (span asli tidak diubah karena masih dipakai bagian SDK lain).
func filterAttrs(
	in []attribute.KeyValue,
) []attribute.KeyValue {
	// kapasitas awal sebesar masukan agar tidak berkali-kali realokasi
	out := make([]attribute.KeyValue, 0, len(in))
	for _, kv := range in {
		// kunci terlarang: tidak diekspor
		if denied(string(kv.Key)) {
			continue
		}
		// nilai string disamarkan; tipe lain (angka, bool) tidak mungkin memuat token, jadi dibiarkan
		if kv.Value.Type() == attribute.STRING {
			kv = attribute.String(string(kv.Key), scrub(kv.Value.AsString()))
		}
		out = append(out, kv)
	}
	return out
}

// redactedSpan membungkus ReadOnlySpan dengan atribut, event, dan status yang sudah disaring. Metode lain
// (nama, waktu, ID, induk) diteruskan apa adanya lewat penyematan antarmuka.
type redactedSpan struct {
	sdktrace.ReadOnlySpan
	// attrs: atribut span setelah disaring.
	attrs []attribute.KeyValue
	// events: event span (mis. exception) setelah atributnya disaring.
	events []sdktrace.Event
	// status: status span dengan deskripsi yang sudah disamarkan.
	status sdktrace.Status
}

// Attributes mengembalikan atribut yang sudah disaring (menimpa metode asli).
func (s redactedSpan) Attributes() []attribute.KeyValue { return s.attrs }

// Events mengembalikan event yang atributnya sudah disaring (menimpa metode asli).
func (s redactedSpan) Events() []sdktrace.Event { return s.events }

// Status mengembalikan status dengan deskripsi yang sudah disamarkan (menimpa metode asli).
func (s redactedSpan) Status() sdktrace.Status { return s.status }

// redactingExporter menyaring semua span sebelum diteruskan ke exporter asli. Ini pagar privasi di sisi aplikasi:
// path mentah, query string, header, dan alamat klien tidak pernah meninggalkan proses, dan nilai yang tersisa
// disamarkan dari bentuk rahasia.
type redactingExporter struct {
	// next: exporter asli (OTLP gRPC) yang menerima span yang sudah bersih.
	next sdktrace.SpanExporter
}

// newRedactingExporter membungkus exporter dengan penyaring atribut.
func newRedactingExporter(
	next sdktrace.SpanExporter,
) sdktrace.SpanExporter {
	return &redactingExporter{next: next}
}

// ExportSpans menyaring setiap span lalu meneruskannya ke exporter asli.
func (e *redactingExporter) ExportSpans(
	ctx context.Context,
	spans []sdktrace.ReadOnlySpan,
) error {
	// salinan daftar span yang sudah disaring
	out := make([]sdktrace.ReadOnlySpan, len(spans))
	for i, s := range spans {
		// salin event dengan atribut yang sudah disaring (exception.message bisa memuat teks galat)
		events := make([]sdktrace.Event, len(s.Events()))
		for j, ev := range s.Events() {
			ev.Attributes = filterAttrs(ev.Attributes)
			events[j] = ev
		}
		// deskripsi status bisa memuat teks galat; samarkan
		st := s.Status()
		st.Description = scrub(st.Description)
		// bungkus span asli dengan versi yang sudah bersih
		out[i] = redactedSpan{ReadOnlySpan: s, attrs: filterAttrs(s.Attributes()), events: events, status: st}
	}
	return e.next.ExportSpans(ctx, out)
}

// Shutdown meneruskan ke exporter asli.
func (e *redactingExporter) Shutdown(ctx context.Context) error { return e.next.Shutdown(ctx) }
