package dto

import (
	"qms/internal/model"
	"strings"
	"time"
)

// DefaultTicketPrefix dipakai bila owner tidak mengisi prefix.
const DefaultTicketPrefix = "A"

// NormalizeTicketPrefix: huruf besar, 1-3 karakter A-Z/0-9. Kosong dikembalikan apa adanya (ok=true).
func NormalizeTicketPrefix(raw string) (string, bool) {
	// Buang spasi di tepi dan ubah ke huruf besar supaya "a1 " dan "A1" dianggap sama.
	p := strings.ToUpper(strings.TrimSpace(raw))
	// Prefix kosong: tidak diisi owner, jadi dikembalikan apa adanya dan dianggap valid.
	if p == "" {
		return "", true
	}
	// Prefix lebih dari 3 karakter: tidak sesuai aturan, tolak.
	if len(p) > 3 {
		return "", false
	}
	// Periksa setiap karakter: hanya huruf A-Z dan angka 0-9 yang diizinkan.
	for _, r := range p {
		// Karakter bukan huruf besar maupun angka: prefix tidak valid, tolak.
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", false
		}
	}
	// Semua karakter lolos pemeriksaan: kembalikan prefix yang sudah dirapikan.
	return p, true
}

// CreateQueueRequest: body POST untuk membuat antrian. Nama dan deskripsi tidak boleh berisi tag HTML.
type CreateQueueRequest struct {
	// QueueName: nama antrian. Wajib diisi.
	QueueName string `json:"queue_name" binding:"required,no_html_tags"`
	// QueueDescription: deskripsi antrian yang ditampilkan ke customer. Boleh kosong.
	QueueDescription string `json:"queue_description" binding:"no_html_tags"`
	// Opsional: 1-3 huruf/angka. Kosong = "A" (create) / tidak diubah (update).
	TicketPrefix string `json:"ticket_prefix"`
}

// UpdateQueueRequest: body PUT untuk mengubah antrian. Bentuknya sama dengan CreateQueueRequest.
type UpdateQueueRequest struct {
	// QueueName: nama antrian yang baru. Wajib diisi.
	QueueName string `json:"queue_name" binding:"required,no_html_tags"`
	// QueueDescription: deskripsi antrian yang baru. Boleh kosong.
	QueueDescription string `json:"queue_description" binding:"no_html_tags"`
	// Opsional: 1-3 huruf/angka. Kosong = "A" (create) / tidak diubah (update).
	TicketPrefix string `json:"ticket_prefix"`
}

// QueueResponse: antrian lengkap untuk owner, termasuk prefix dan waktu tutup.
type QueueResponse struct {
	// ID: id antrian di database.
	ID int64 `json:"id"`
	// OwnerID: id owner (user) yang memiliki antrian.
	OwnerID int64 `json:"owner_id"`
	// QueueName: nama antrian.
	QueueName string `json:"queue_name"`
	// QueueDesc: deskripsi antrian. Null bila tidak diisi.
	QueueDesc *string `json:"queue_desc"`
	// TicketPrefix: prefix nomor tiket (mis. "A" di "A-007").
	TicketPrefix string `json:"ticket_prefix"`
	// Status: status antrian: active atau closed.
	Status string `json:"status"`
	// CreatedAt: waktu antrian dibuat.
	CreatedAt time.Time `json:"created_at"`
	// ClosedAt: waktu antrian ditutup. Null bila masih aktif.
	ClosedAt *time.Time `json:"closed_at"`
}

// NewQueueResponse membuat QueueResponse dari model.Queue. Deskripsi kosong dan waktu tutup yang belum ada dikirim
// sebagai null.
func NewQueueResponse(q model.Queue) QueueResponse {
	// Deskripsi antrian disimpan sebagai pointer agar bisa dikirim null.
	var desc *string
	// Deskripsi ada di database (tidak null): simpan alamat isinya.
	if q.QueueDesc.Valid {
		desc = &q.QueueDesc.String
	}
	// Susun respons dari field model; deskripsi dan waktu tutup sudah dalam bentuk pointer.
	return QueueResponse{
		ID:           q.ID,
		OwnerID:      q.OwnerID,
		QueueName:    q.QueueName,
		QueueDesc:    desc,
		TicketPrefix: q.TicketPrefix,
		Status:       q.Status,
		CreatedAt:    q.CreatedAt,
		// nullTimePtr: waktu tutup, kosong bila antrian belum ditutup.
		ClosedAt: nullTimePtr(q.ClosedAt),
	}
}

// PublicQueueResponse: antrian untuk publik (customer, layar). Tanpa owner_id.
type PublicQueueResponse struct {
	// ID: id antrian di database.
	ID int64 `json:"id"`
	// QueueName: nama antrian.
	QueueName string `json:"queue_name"`
	// QueueDesc: deskripsi antrian. Null bila tidak diisi.
	QueueDesc *string `json:"queue_desc"`
	// TicketPrefix: prefix nomor tiket (mis. "A" di "A-007").
	TicketPrefix string `json:"ticket_prefix"`
	// Status: status antrian: active atau closed.
	Status string `json:"status"`
	// CreatedAt: waktu antrian dibuat.
	CreatedAt time.Time `json:"created_at"`
	// ClosedAt: waktu antrian ditutup. Null bila masih aktif.
	ClosedAt *time.Time `json:"closed_at"`
}

// NewPublicQueueResponse membuat PublicQueueResponse dengan menyaring QueueResponse lengkap (tanpa owner_id).
func NewPublicQueueResponse(q model.Queue) PublicQueueResponse {
	// Bentuk versi lengkap dulu, lalu salin field yang boleh tampil ke publik.
	full := NewQueueResponse(q)
	// Salin field publik dari versi lengkap; OwnerID tidak ikut disalin.
	return PublicQueueResponse{
		ID:           full.ID,
		QueueName:    full.QueueName,
		QueueDesc:    full.QueueDesc,
		TicketPrefix: full.TicketPrefix,
		Status:       full.Status,
		CreatedAt:    full.CreatedAt,
		ClosedAt:     full.ClosedAt,
	}
}

// NewQueueResponses mengubah daftar antrian menjadi daftar QueueResponse, dengan urutan yang sama.
func NewQueueResponses(queues []model.Queue) []QueueResponse {
	// Buat slice kosong dengan kapasitas sesuai jumlah antrian.
	out := make([]QueueResponse, 0, len(queues))
	// Ubah setiap antrian dan tambahkan ke hasil.
	for _, q := range queues {
		// Ubah satu antrian menjadi QueueResponse lalu tambahkan ke hasil.
		out = append(out, NewQueueResponse(q))
	}
	return out
}

// QueueQRResponse: respons endpoint QR owner.
// `action` memakai nama yang sama pola action lain ("queue_qr").
type QueueQRResponse struct {
	// Action: nama aksi untuk klien, selalu "queue_qr".
	Action string `json:"action"`
	// QueueID: id antrian yang QR-nya dibuat.
	QueueID int64 `json:"queue_id"`
	// Type: jenis QR: static (kode tetap) atau dynamic (kode berganti otomatis).
	Type string `json:"type"`
	// JoinURL: tautan yang dipindai customer untuk bergabung ke antrian.
	JoinURL string `json:"join_url"`
	// ExpiresAt: waktu kedaluwarsa dalam detik Unix. Null untuk QR statis yang tidak punya batas waktu.
	ExpiresAt *int64 `json:"expires_at"`
}

// NewQueueQRResponse membuat QueueQRResponse dari nilai yang sudah dihitung service. Waktu kedaluwarsa nil untuk QR
// statis.
func NewQueueQRResponse(
	action string,
	queueID int64,
	qrType string,
	joinURL string,
	expiresAt *int64,
) QueueQRResponse {
	// Susun respons langsung dari argumen; tidak ada perhitungan atau pengecekan di sini.
	return QueueQRResponse{
		Action:    action,
		QueueID:   queueID,
		Type:      qrType,
		JoinURL:   joinURL,
		ExpiresAt: expiresAt,
	}
}

// JoinByTokenRequest: body untuk bergabung ke antrian lewat token QR.
type JoinByTokenRequest struct {
	// Token join dari QR: format `v1.<payload>.<sig>`; JSON key-nya `t` (tautan `/join?t=...`).
	Token string `json:"t" binding:"required"`
}

// SetQRTypeRequest: body untuk mengganti mode QR (statis atau dinamis).
type SetQRTypeRequest struct {
	// Type: mode QR yang diinginkan: static atau dynamic.
	Type string `json:"type" binding:"required"`
}

// NextTicketRequest: body Next, Skip, dan Recall. `counter_id` opsional (0 = counter default). `expected_ticket_id`
// opsional: nomor yang dilihat perangkat sebagai nomor counter ini (0 = counter kosong). Nil = klien lama, tanpa
// pengecekan (D9).
type NextTicketRequest struct {
	// CounterID: counter yang memanggil nomor berikutnya. 0 berarti counter default antrian.
	CounterID int64 `json:"counter_id"`
	// ExpectedTicketID: nomor yang dilihat perangkat di counter ini. Nil = klien lama, tanpa pengecekan.
	ExpectedTicketID *int64 `json:"expected_ticket_id"`
}

// SkipSettingsRequest: body PUT /queues/:id/settings/skip.
type SkipSettingsRequest struct {
	// Fallback: ke mana nomor yang dilewati kembali: remove (dikeluarkan), back (kembali ke belakang), atau after_n.
	Fallback string `json:"fallback"`
	// N: posisi kembali untuk after_n (mis. 3 = masuk di urutan ke-4). Diabaikan untuk fallback lain.
	N int `json:"n"`
	// MaxReturns: berapa kali nomor boleh kembali sebelum dilewati permanen.
	MaxReturns int `json:"max_returns"`
}

// SkipSettingsResponse: pengaturan skip antrian. N null bila tidak dipakai (nilainya 0).
type SkipSettingsResponse struct {
	// Fallback: ke mana nomor yang dilewati kembali: remove, back, atau after_n.
	Fallback string `json:"fallback"`
	// N: posisi kembali untuk after_n. Null bila tidak dipakai.
	N *int `json:"n"`
	// MaxReturns: berapa kali nomor boleh kembali sebelum dilewati permanen.
	MaxReturns int `json:"max_returns"`
}

// NewSkipSettingsResponse membuat SkipSettingsResponse dari model. N bernilai 0 dikirim sebagai null.
func NewSkipSettingsResponse(s model.SkipSettings) SkipSettingsResponse {
	// Nilai N disimpan sebagai pointer agar 0 bisa dikirim sebagai null.
	var n *int
	// N lebih dari 0 berarti fallback after_n dipakai: simpan alamat nilainya.
	if s.N > 0 {
		// Salin nilai N ke variabel baru agar pointer tidak menunjuk ke field model.
		v := s.N
		n = &v
	}
	// Susun respons dengan fallback, N (pointer), dan batas kembali.
	return SkipSettingsResponse{Fallback: s.Fallback, N: n, MaxReturns: s.MaxReturns}
}
