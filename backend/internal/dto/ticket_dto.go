package dto

import (
	"database/sql"
	"qms/internal/model"
	"time"
)

// TicketResponse: satu tiket untuk customer dan owner. Waktu yang kosong di database dikirim sebagai null.
type TicketResponse struct {
	// ID: id tiket di database.
	ID int64 `json:"id"`
	// QueueID: id antrian tempat tiket ini berada.
	QueueID int64 `json:"queue_id"`
	// TicketNumber: nomor tiket yang dilihat customer (tanpa prefix).
	TicketNumber int `json:"ticket_number"`
	// SortKey: urutan tiket di dalam antrian. Nilai lebih kecil didahulukan.
	SortKey int64 `json:"sort_key"`
	// UserID: id customer pemilik tiket.
	UserID int64 `json:"user_id"`
	// Status: status tiket: waiting, currently_served, completed, skipped, cancelled, atau deleted.
	Status string `json:"status"`
	// SkipCount: berapa kali nomor ini sudah dilewati (skip).
	SkipCount int `json:"skip_count"`
	// LastNotifiedAt: waktu terakhir customer diberi tahu. Null bila belum pernah.
	LastNotifiedAt *time.Time `json:"last_notified_at"`
	// CreatedAt: waktu tiket dibuat (customer bergabung ke antrian).
	CreatedAt time.Time `json:"created_at"`
	// CompletedAt: waktu nomor selesai dilayani. Null bila belum selesai.
	CompletedAt *time.Time `json:"completed_at"`
	// CancelledAt: waktu tiket dibatalkan customer. Null bila tidak dibatalkan.
	CancelledAt *time.Time `json:"cancelled_at"`
	// CalledCounterID: id counter yang memanggil nomor ini. Null bila belum pernah dipanggil.
	CalledCounterID *int64 `json:"called_counter_id"`
	// CalledCounterName: nama counter yang memanggil. Null untuk counter default atau bila belum dipanggil.
	CalledCounterName *string `json:"called_counter_name"`
	// CalledAt: waktu nomor terakhir dipanggil. Null bila belum pernah dipanggil.
	CalledAt *time.Time `json:"called_at"`
}

// NewTicketResponse membuat TicketResponse dari model.Ticket. Field waktu yang nullable diubah ke pointer,
// dan counter_id 0 dikirim sebagai null.
func NewTicketResponse(t model.Ticket) TicketResponse {
	// Salin field biasa apa adanya, lalu ubah field nullable lewat helper di bawah.
	return TicketResponse{
		ID:           t.ID,
		QueueID:      t.QueueID,
		TicketNumber: t.TicketNumber,
		SortKey:      t.SortKey,
		UserID:       t.UserID,
		Status:       t.Status,
		SkipCount:    t.SkipCount,
		// nullTimePtr: sql.NullTime yang tidak valid menjadi nil, yang valid menjadi pointer ke waktunya.
		LastNotifiedAt: nullTimePtr(t.LastNotifiedAt),
		CreatedAt:      t.CreatedAt,
		// nullTimePtr: waktu selesai, kosong bila tiket belum selesai.
		CompletedAt: nullTimePtr(t.CompletedAt),
		// nullTimePtr: waktu batal, kosong bila tiket tidak dibatalkan.
		CancelledAt: nullTimePtr(t.CancelledAt),
		// Closure: counter_id 0 berarti tidak ada counter, jadi dikirim sebagai null; selain itu dibuat pointer.
		CalledCounterID: func() *int64 {
			// Tidak ada counter yang memanggil tiket ini: kembalikan nil.
			if t.CalledCounterID == 0 {
				return nil
			}
			// Salin nilainya ke variabel baru agar pointer yang dikembalikan tidak menunjuk ke field model.
			v := t.CalledCounterID
			return &v
		}(),
		// nullTimePtr: waktu dipanggil, kosong bila tiket belum pernah dipanggil.
		CalledAt: nullTimePtr(t.CalledAt),
	}
}

// TicketEvent: payload event real-time untuk satu tiket. Tanpa user_id & sort_key: penerima event bisa customer
// lain di queue yang sama.
type TicketEvent struct {
	// ID: id tiket di database.
	ID int64 `json:"id"`
	// QueueID: id antrian tempat tiket ini berada.
	QueueID int64 `json:"queue_id"`
	// TicketNumber: nomor tiket yang dilihat customer (tanpa prefix).
	TicketNumber int `json:"ticket_number"`
	// Status: status tiket saat event dikirim.
	Status string `json:"status"`
	// SkipCount: berapa kali nomor ini sudah dilewati (skip).
	SkipCount int `json:"skip_count"`
	// CreatedAt: waktu tiket dibuat.
	CreatedAt time.Time `json:"created_at"`
	// CompletedAt: waktu nomor selesai dilayani. Null bila belum selesai.
	CompletedAt *time.Time `json:"completed_at"`
	// CancelledAt: waktu tiket dibatalkan. Null bila tidak dibatalkan.
	CancelledAt *time.Time `json:"cancelled_at"`
	// CalledCounterID: id counter yang memanggil nomor ini. Null bila belum pernah dipanggil.
	CalledCounterID *int64 `json:"called_counter_id"`
	// CalledCounterName: nama counter yang memanggil. Null untuk counter default atau bila belum dipanggil.
	CalledCounterName *string `json:"called_counter_name"`
	// CalledAt: waktu nomor terakhir dipanggil. Null bila belum pernah dipanggil.
	CalledAt *time.Time `json:"called_at"`
}

// NewTicketEvent membuat TicketEvent dari model.Ticket dan menyaring field yang tidak boleh sampai ke customer lain
// (user_id dan sort_key).
func NewTicketEvent(t model.Ticket) TicketEvent {
	// Salin field yang aman dikirim ke semua penerima event, lalu ubah field nullable lewat helper.
	return TicketEvent{
		ID:           t.ID,
		QueueID:      t.QueueID,
		TicketNumber: t.TicketNumber,
		Status:       t.Status,
		SkipCount:    t.SkipCount,
		CreatedAt:    t.CreatedAt,
		// nullTimePtr: waktu selesai, kosong bila tiket belum selesai.
		CompletedAt: nullTimePtr(t.CompletedAt),
		// nullTimePtr: waktu batal, kosong bila tiket tidak dibatalkan.
		CancelledAt: nullTimePtr(t.CancelledAt),
		// Closure: counter_id 0 berarti tidak ada counter, jadi dikirim sebagai null; selain itu dibuat pointer.
		CalledCounterID: func() *int64 {
			// Tidak ada counter yang memanggil tiket ini: kembalikan nil.
			if t.CalledCounterID == 0 {
				return nil
			}
			// Salin nilainya ke variabel baru agar pointer yang dikembalikan tidak menunjuk ke field model.
			v := t.CalledCounterID
			return &v
		}(),
		// counterNamePtr: nama counter kosong (counter default) dikirim sebagai null.
		CalledCounterName: counterNamePtr(t.CounterName),
		// nullTimePtr: waktu dipanggil, kosong bila tiket belum pernah dipanggil.
		CalledAt: nullTimePtr(t.CalledAt),
	}
}

// OwnerTicketResponse: tiket untuk daftar tiket milik owner. Menambah catatan owner di atas TicketResponse.
// Hanya endpoint owner yang boleh memakainya; respons untuk customer tetap TicketResponse tanpa catatan.
type OwnerTicketResponse struct {
	// TicketResponse: seluruh field tiket, disematkan langsung ke JSON.
	TicketResponse
	// Note: catatan owner untuk nomor ini. Null bila belum ada catatan.
	Note *string `json:"note"`
	// NoteUpdatedAt: waktu catatan terakhir diubah. Null bila belum ada catatan.
	NoteUpdatedAt *time.Time `json:"note_updated_at"`
}

// NewOwnerTicketResponse membuat OwnerTicketResponse: isi TicketResponse, lalu catatan owner bila ada.
func NewOwnerTicketResponse(t model.Ticket) OwnerTicketResponse {
	// Bungkus tiket menjadi TicketResponse, lalu tambahkan field catatan owner di atasnya.
	out := OwnerTicketResponse{TicketResponse: NewTicketResponse(t)}
	// Catatan ada (tidak null di database): simpan alamat isinya.
	if t.Note.Valid {
		out.Note = &t.Note.String
	}
	// Waktu catatan terakhir diubah ada: simpan alamat waktunya.
	if t.NoteUpdatedAt.Valid {
		out.NoteUpdatedAt = &t.NoteUpdatedAt.Time
	}
	return out
}

// NewOwnerTicketResponses mengubah daftar tiket menjadi daftar OwnerTicketResponse, dengan urutan yang sama.
func NewOwnerTicketResponses(tickets []model.Ticket) []OwnerTicketResponse {
	// Buat slice kosong dengan kapasitas sesuai jumlah tiket, supaya tidak perlu alokasi ulang saat append.
	out := make([]OwnerTicketResponse, 0, len(tickets))
	// Ubah setiap tiket dan tambahkan ke hasil.
	for _, t := range tickets {
		// Ubah satu tiket menjadi OwnerTicketResponse lalu tambahkan ke hasil.
		out = append(out, NewOwnerTicketResponse(t))
	}
	return out
}

// NewTicketResponses mengubah daftar tiket menjadi daftar TicketResponse, dengan urutan yang sama.
func NewTicketResponses(tickets []model.Ticket) []TicketResponse {
	// Buat slice kosong dengan kapasitas sesuai jumlah tiket, supaya tidak perlu alokasi ulang saat append.
	out := make([]TicketResponse, 0, len(tickets))
	// Ubah setiap tiket dan tambahkan ke hasil.
	for _, t := range tickets {
		// Ubah satu tiket menjadi TicketResponse lalu tambahkan ke hasil.
		out = append(out, NewTicketResponse(t))
	}
	return out
}

// nullTimePtr mengubah sql.NullTime menjadi pointer: nil bila tidak valid (null di database), selain itu pointer ke waktunya.
func nullTimePtr(t sql.NullTime) *time.Time {
	// Nilai NULL di database: kembalikan nil agar JSON menjadi null.
	if !t.Valid {
		return nil
	}
	// Nilai ada: kembalikan alamat waktunya.
	return &t.Time
}

// counterNamePtr mengubah nama counter menjadi pointer. Nama kosong (counter default) dikirim sebagai null.
func counterNamePtr(name string) *string {
	// Nama kosong berarti counter default atau tidak ada counter: kembalikan nil.
	if name == "" {
		return nil
	}
	// Nama ada: kembalikan alamat namanya.
	return &name
}

// QueueStatusResponse: status antrian untuk layar customer dan owner: nomor yang sedang dilayani, nomor sebelum dan
// sesudah, serta posisi tiket milik requester.
type QueueStatusResponse struct {
	// Queue: ringkasan antrian untuk publik (tanpa owner_id).
	Queue PublicQueueResponse `json:"queue"`
	// MyTicket: tiket milik requester. Null bila requester tidak punya tiket di antrian ini.
	MyTicket *TicketResponse `json:"my_ticket"`
	// CurrentNumber: nomor yang paling baru dipanggil. Null bila tidak ada counter yang sedang melayani.
	CurrentNumber *int `json:"current_number"`
	// CurrentCounterName: nama counter yang memanggil CurrentNumber. Null untuk counter default.
	CurrentCounterName *string `json:"current_counter_name"`
	// WaitingAhead: jumlah tiket waiting di depan tiket requester. Null bila requester tidak sedang menunggu.
	WaitingAhead *int `json:"waiting_ahead"`
	// PreviousNumber: nomor yang terakhir selesai dilayani. Null bila belum ada.
	PreviousNumber *int `json:"previous_number"`
	// NextNumber: nomor waiting pertama yang akan dipanggil berikutnya. Null bila antrian kosong.
	NextNumber *int `json:"next_number"`
	// Serving: daftar counter yang sedang melayani nomor (satu entri per counter).
	Serving []ServingEntryResponse `json:"serving"`
}

// ServingEntryResponse: satu counter yang sedang melayani nomor (status antrian).
type ServingEntryResponse struct {
	// CounterID: id counter yang sedang melayani.
	CounterID int64 `json:"counter_id"`
	// CounterName: nama counter. Null untuk counter default.
	CounterName *string `json:"counter_name"`
	// TicketNumber: nomor yang sedang dilayani counter ini.
	TicketNumber int `json:"ticket_number"`
}

// NewQueueStatusResponse membuat QueueStatusResponse dari model.QueueStatus. Tiket milik requester dan nama counter
// yang sedang melayani dikirim sebagai null bila tidak ada.
func NewQueueStatusResponse(s model.QueueStatus) QueueStatusResponse {
	// Tiket milik requester: bila ada, ubah menjadi TicketResponse dan simpan alamatnya.
	var mine *TicketResponse
	// requester punya tiket di antrian ini: ubah tiketnya.
	if s.MyTicket != nil {
		// Ubah tiket milik requester menjadi TicketResponse.
		t := NewTicketResponse(*s.MyTicket)
		// Simpan alamat hasilnya agar bisa dikirim sebagai null bila tidak ada.
		mine = &t
	}
	// Nama counter yang sedang melayani: disalin bila ada.
	var currentCounterName *string
	// Nama counter ada (bukan counter default): salin ke field respons.
	if s.CurrentCounterName != nil {
		currentCounterName = s.CurrentCounterName
	}
	// Susun respons: antrian dibuat versi publiknya, daftar counter diubah, dan field lain disalin apa adanya.
	return QueueStatusResponse{
		// NewPublicQueueResponse: versi antrian yang aman untuk publik (tanpa data owner).
		Queue:              NewPublicQueueResponse(s.Queue),
		MyTicket:           mine,
		CurrentNumber:      s.CurrentNumber,
		CurrentCounterName: currentCounterName,
		WaitingAhead:       s.WaitingAhead,
		PreviousNumber:     s.PreviousNumber,
		NextNumber:         s.NextNumber,
		// servingResponses: ubah setiap counter yang sedang melayani menjadi ServingEntryResponse.
		Serving: servingResponses(s.Serving),
	}
}

// servingResponses mengubah daftar counter yang sedang melayani menjadi ServingEntryResponse, dengan urutan yang sama.
func servingResponses(entries []model.ServingEntry) []ServingEntryResponse {
	// Buat slice kosong dengan kapasitas sesuai jumlah entri.
	out := make([]ServingEntryResponse, 0, len(entries))
	// Salin setiap entri ke bentuk respons.
	for _, e := range entries {
		// Salin counter_id, nama counter, dan nomor tiket ke ServingEntryResponse, lalu tambahkan ke hasil.
		out = append(out, ServingEntryResponse{CounterID: e.CounterID, CounterName: e.CounterName, TicketNumber: e.TicketNumber})
	}
	return out
}

// UserTicketResponse: tiket saya + info antriannya.
type UserTicketResponse struct {
	// TicketResponse: seluruh field tiket, disematkan langsung ke JSON.
	TicketResponse
	// QueueName: nama antrian tempat tiket ini berada.
	QueueName string `json:"queue_name"`
	// TicketPrefix: prefix nomor antrian (mis. "A" di "A-007").
	TicketPrefix string `json:"ticket_prefix"`
	// QueueStatus: status antrian: active atau closed.
	QueueStatus string `json:"queue_status"`
	// CurrentNumber: nomor yang sedang dilayani di antrian ini. Null bila tidak ada yang dilayani.
	CurrentNumber *int `json:"current_number"`
}

// NewUserTicketResponses mengubah daftar tiket milik customer, beserta info antriannya, menjadi UserTicketResponse.
func NewUserTicketResponses(tickets []model.UserTicket) []UserTicketResponse {
	// Buat slice kosong dengan kapasitas sesuai jumlah tiket.
	out := make([]UserTicketResponse, 0, len(tickets))
	// Ubah setiap tiket milik user dan tambahkan ke hasil.
	for _, t := range tickets {
		// Gabungkan tiket (lewat NewTicketResponse) dengan info antriannya ke satu UserTicketResponse.
		out = append(out, UserTicketResponse{
			// NewTicketResponse: bagian tiket yang sama dengan respons tiket biasa.
			TicketResponse: NewTicketResponse(t.Ticket),
			QueueName:      t.QueueName,
			TicketPrefix:   t.TicketPrefix,
			QueueStatus:    t.QueueStatus,
			CurrentNumber:  t.CurrentNumber,
		})
	}
	return out
}
