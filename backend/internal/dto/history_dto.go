package dto

import (
	"time"

	"qms/internal/model"
)

// HistoryQueueResponse: ringkasan antrian yang sudah ditutup. Hitungan per hasil akhir menjumlah ke Total.
type HistoryQueueResponse struct {
	// ID: id antrian di database.
	ID int64 `json:"id"`
	// QueueName: nama antrian saat ditutup.
	QueueName string `json:"queue_name"`
	// TicketPrefix: prefix nomor tiket antrian ini.
	TicketPrefix string `json:"ticket_prefix"`
	// OpenedAt: waktu antrian dibuat.
	OpenedAt time.Time `json:"opened_at"`
	// ClosedAt: waktu antrian ditutup.
	ClosedAt time.Time `json:"closed_at"`
	// Total: jumlah seluruh nomor yang pernah masuk antrian ini.
	Total int `json:"total"`
	// Completed: jumlah nomor yang selesai dilayani.
	Completed int `json:"completed"`
	// Skipped: jumlah nomor yang dilewati permanen.
	Skipped int `json:"skipped"`
	// Cancelled: jumlah nomor yang dibatalkan customer.
	Cancelled int `json:"cancelled"`
	// Deleted: jumlah nomor yang dihapus owner dari antrian.
	Deleted int `json:"deleted"`
	// Unserved: jumlah nomor yang masih menunggu atau sedang dilayani saat antrian ditutup.
	Unserved int `json:"unserved"`
}

// NewHistoryQueueResponse membuat HistoryQueueResponse dari model.HistoryQueue dengan menyalin field-nya apa adanya.
func NewHistoryQueueResponse(q model.HistoryQueue) HistoryQueueResponse {
	// Salin semua field ringkasan antrian (termasuk hitungan per hasil akhir) ke respons.
	return HistoryQueueResponse{
		ID:           q.ID,
		QueueName:    q.QueueName,
		TicketPrefix: q.TicketPrefix,
		OpenedAt:     q.OpenedAt,
		ClosedAt:     q.ClosedAt,
		Total:        q.Total,
		Completed:    q.Completed,
		Skipped:      q.Skipped,
		Cancelled:    q.Cancelled,
		Deleted:      q.Deleted,
		Unserved:     q.Unserved,
	}
}

// HistoryTicketResponse: satu nomor di antrian yang sudah ditutup. Dibaca owner saja, jadi memuat email
// customer dan catatan owner.
type HistoryTicketResponse struct {
	// TicketID: id tiket di database.
	TicketID int64 `json:"ticket_id"`
	// TicketNumber: nomor tiket yang dilihat customer (tanpa prefix).
	TicketNumber int `json:"ticket_number"`
	// Outcome: hasil akhir nomor: completed, skipped, cancelled, deleted, atau unserved.
	Outcome string `json:"outcome"`
	// SkipCount: berapa kali nomor ini sudah dilewati.
	SkipCount int `json:"skip_count"`
	// JoinedAt: waktu customer bergabung ke antrian.
	JoinedAt time.Time `json:"joined_at"`
	// CalledAt: waktu nomor dipanggil. Null bila belum pernah dipanggil.
	CalledAt *time.Time `json:"called_at"`
	// CompletedAt: waktu nomor selesai dilayani. Null bila belum selesai.
	CompletedAt *time.Time `json:"completed_at"`
	// CancelledAt: waktu tiket dibatalkan. Null bila tidak dibatalkan.
	CancelledAt *time.Time `json:"cancelled_at"`
	// CustomerName: nama customer pemilik tiket.
	CustomerName string `json:"customer_name"`
	// CustomerEmail: email customer pemilik tiket.
	CustomerEmail string `json:"customer_email"`
	// CounterName: nama counter yang memanggil nomor. Null untuk counter default atau bila belum dipanggil.
	CounterName *string `json:"counter_name"`
	// Note: catatan owner untuk nomor ini. Null bila belum ada catatan.
	Note *string `json:"note"`
}

// NewHistoryTicketResponse membuat HistoryTicketResponse dari model.HistoryTicket. Field waktu, nama counter, dan
// catatan yang nullable diubah ke pointer hanya bila nilainya ada.
func NewHistoryTicketResponse(t model.HistoryTicket) HistoryTicketResponse {
	// Salin field yang selalu ada; outcome diubah dari status tiket ke bentuk hasil akhir.
	out := HistoryTicketResponse{
		TicketID:     t.TicketID,
		TicketNumber: t.TicketNumber,
		// model.HistoryOutcome: menerjemahkan status tiket menjadi hasil akhir (completed, skipped, dll).
		Outcome:       model.HistoryOutcome(t.Status),
		SkipCount:     t.SkipCount,
		JoinedAt:      t.JoinedAt,
		CustomerName:  t.CustomerName,
		CustomerEmail: t.CustomerEmail,
	}
	// Waktu dipanggil ada: simpan alamatnya.
	if t.CalledAt.Valid {
		out.CalledAt = &t.CalledAt.Time
	}
	// Waktu selesai ada: simpan alamatnya.
	if t.CompletedAt.Valid {
		out.CompletedAt = &t.CompletedAt.Time
	}
	// Waktu batal ada: simpan alamatnya.
	if t.CancelledAt.Valid {
		out.CancelledAt = &t.CancelledAt.Time
	}
	// Nama counter ada: simpan alamat namanya.
	if t.CounterName.Valid {
		out.CounterName = &t.CounterName.String
	}
	// Catatan owner ada: simpan alamat isinya.
	if t.Note.Valid {
		out.Note = &t.Note.String
	}
	return out
}

// HistoryQueuesResponse: GET /history/queues. NextCursor null = halaman terakhir.
type HistoryQueuesResponse struct {
	// Action: nama aksi untuk klien, selalu "list_history_queues".
	Action string `json:"action"`
	// Queues: ringkasan antrian di halaman ini.
	Queues []HistoryQueueResponse `json:"queues"`
	// NextCursor: kursor untuk halaman berikutnya. Null bila ini halaman terakhir.
	NextCursor *string `json:"next_cursor"`
}

// NewHistoryQueuesResponse membuat satu halaman riwayat antrian. Cursor kosong dikirim sebagai null (halaman terakhir).
func NewHistoryQueuesResponse(queues []model.HistoryQueue, nextCursor string) HistoryQueuesResponse {
	// Siapkan respons dengan action dan slice kosong dengan kapasitas sesuai jumlah antrian.
	out := HistoryQueuesResponse{
		Action: "list_history_queues",
		Queues: make([]HistoryQueueResponse, 0, len(queues)),
	}
	// Ubah setiap antrian di halaman ini dan tambahkan ke hasil.
	for _, q := range queues {
		// Ubah satu antrian menjadi HistoryQueueResponse lalu tambahkan ke hasil.
		out.Queues = append(out.Queues, NewHistoryQueueResponse(q))
	}
	// Ada cursor (masih ada halaman berikutnya): simpan alamatnya.
	if nextCursor != "" {
		out.NextCursor = &nextCursor
	}
	return out
}

// HistoryQueueDetailResponse: GET /history/queues/:queue_id.
type HistoryQueueDetailResponse struct {
	// Action: nama aksi untuk klien, selalu "history_queue".
	Action string `json:"action"`
	// Queue: ringkasan antrian yang dibuka.
	Queue HistoryQueueResponse `json:"queue"`
	// Tickets: satu halaman nomor di antrian ini.
	Tickets []HistoryTicketResponse `json:"tickets"`
	// NextCursor: kursor untuk halaman nomor berikutnya. Null bila ini halaman terakhir.
	NextCursor *string `json:"next_cursor"`
}

// NewHistoryQueueDetailResponse membuat detail satu antrian riwayat: ringkasannya dan satu halaman nomornya.
func NewHistoryQueueDetailResponse(queue model.HistoryQueue, tickets []model.HistoryTicket, nextCursor string) HistoryQueueDetailResponse {
	// Siapkan respons dengan ringkasan antrian dan slice kosong untuk nomor-nomornya.
	out := HistoryQueueDetailResponse{
		Action: "history_queue",
		// NewHistoryQueueResponse: ubah ringkasan antrian ke bentuk respons.
		Queue:   NewHistoryQueueResponse(queue),
		Tickets: make([]HistoryTicketResponse, 0, len(tickets)),
	}
	// Ubah setiap nomor di halaman ini dan tambahkan ke hasil.
	for _, t := range tickets {
		// Ubah satu nomor menjadi HistoryTicketResponse lalu tambahkan ke hasil.
		out.Tickets = append(out.Tickets, NewHistoryTicketResponse(t))
	}
	// Ada cursor (masih ada halaman berikutnya): simpan alamatnya.
	if nextCursor != "" {
		out.NextCursor = &nextCursor
	}
	return out
}

// UserHistoryTicketResponse: satu tiket di riwayat customer. Sengaja tanpa catatan owner dan tanpa data user lain.
type UserHistoryTicketResponse struct {
	// TicketID: id tiket di database.
	TicketID int64 `json:"ticket_id"`
	// QueueID: id antrian tempat tiket ini berada.
	QueueID int64 `json:"queue_id"`
	// QueueName: nama antrian tempat tiket ini berada.
	QueueName string `json:"queue_name"`
	// TicketPrefix: prefix nomor antrian (mis. "A" di "A-007").
	TicketPrefix string `json:"ticket_prefix"`
	// TicketNumber: nomor tiket yang dilihat customer (tanpa prefix).
	TicketNumber int `json:"ticket_number"`
	// Outcome: hasil akhir nomor: completed, skipped, cancelled, deleted, atau unserved.
	Outcome string `json:"outcome"`
	// QueueClosed: true bila antrian sudah ditutup.
	QueueClosed bool `json:"queue_closed"`
	// SkipCount: berapa kali nomor ini sudah dilewati.
	SkipCount int `json:"skip_count"`
	// JoinedAt: waktu customer bergabung ke antrian.
	JoinedAt time.Time `json:"joined_at"`
	// CalledAt: waktu nomor dipanggil. Null bila belum pernah dipanggil.
	CalledAt *time.Time `json:"called_at"`
	// CompletedAt: waktu nomor selesai dilayani. Null bila belum selesai.
	CompletedAt *time.Time `json:"completed_at"`
	// CancelledAt: waktu tiket dibatalkan. Null bila tidak dibatalkan.
	CancelledAt *time.Time `json:"cancelled_at"`
	// CounterName: nama counter yang memanggil nomor. Null untuk counter default atau bila belum dipanggil.
	CounterName *string `json:"counter_name"`
}

// NewUserHistoryTicketResponse membuat UserHistoryTicketResponse dari model.UserHistoryTicket. Waktu dan nama counter
// yang nullable diubah ke pointer hanya bila nilainya ada.
func NewUserHistoryTicketResponse(t model.UserHistoryTicket) UserHistoryTicketResponse {
	// Salin field yang selalu ada; outcome diubah dari status tiket, dan QueueClosed dihitung dari status antrian.
	out := UserHistoryTicketResponse{
		TicketID:     t.TicketID,
		QueueID:      t.QueueID,
		QueueName:    t.QueueName,
		TicketPrefix: t.TicketPrefix,
		TicketNumber: t.TicketNumber,
		// model.HistoryOutcome: menerjemahkan status tiket menjadi hasil akhir.
		Outcome: model.HistoryOutcome(t.Status),
		// Antrian sudah ditutup bila statusnya "closed".
		QueueClosed: t.QueueStatus == "closed",
		SkipCount:   t.SkipCount,
		JoinedAt:    t.JoinedAt,
	}
	// Waktu dipanggil ada: simpan alamatnya.
	if t.CalledAt.Valid {
		out.CalledAt = &t.CalledAt.Time
	}
	// Waktu selesai ada: simpan alamatnya.
	if t.CompletedAt.Valid {
		out.CompletedAt = &t.CompletedAt.Time
	}
	// Waktu batal ada: simpan alamatnya.
	if t.CancelledAt.Valid {
		out.CancelledAt = &t.CancelledAt.Time
	}
	// Nama counter ada: simpan alamat namanya.
	if t.CounterName.Valid {
		out.CounterName = &t.CounterName.String
	}
	return out
}

// MyHistoryResponse: GET /users/me/history. NextCursor null = halaman terakhir.
type MyHistoryResponse struct {
	// Action: nama aksi untuk klien, selalu "list_my_history".
	Action string `json:"action"`
	// Tickets: satu halaman tiket milik customer, terbaru dulu.
	Tickets []UserHistoryTicketResponse `json:"tickets"`
	// NextCursor: kursor untuk halaman berikutnya. Null bila ini halaman terakhir.
	NextCursor *string `json:"next_cursor"`
}

// NewMyHistoryResponse membuat satu halaman riwayat tiket milik customer. Cursor kosong dikirim sebagai null.
func NewMyHistoryResponse(tickets []model.UserHistoryTicket, nextCursor string) MyHistoryResponse {
	// Siapkan respons dengan action dan slice kosong dengan kapasitas sesuai jumlah tiket.
	out := MyHistoryResponse{
		Action:  "list_my_history",
		Tickets: make([]UserHistoryTicketResponse, 0, len(tickets)),
	}
	// Ubah setiap tiket di halaman ini dan tambahkan ke hasil.
	for _, t := range tickets {
		// Ubah satu tiket menjadi UserHistoryTicketResponse lalu tambahkan ke hasil.
		out.Tickets = append(out.Tickets, NewUserHistoryTicketResponse(t))
	}
	// Ada cursor (masih ada halaman berikutnya): simpan alamatnya.
	if nextCursor != "" {
		out.NextCursor = &nextCursor
	}
	return out
}
