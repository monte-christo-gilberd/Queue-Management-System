// Package model berisi bentuk data inti aplikasi: baris tabel database (Queue, Ticket, User, dll.) dan tipe
// hasil gabungan yang dibaca dari beberapa query (QueueStatus, DisplaySnapshot, HistoryQueue, dll.).
//
// Aturan yang dipakai di semua model:
//   - Tag `db:"..."` memetakan field ke kolom tabel untuk sqlx. Field dengan `db:"-"` tidak ada di tabel
//     dan diisi oleh aplikasi (mis. Ticket.CounterName).
//   - Kolom yang boleh NULL memakai sql.NullString, sql.NullTime, atau sql.NullInt64, supaya NULL di database
//     bisa dibedakan dari nilai nol (mis. string kosong atau waktu 0001-01-01).
//   - Model tidak punya logika bisnis dan tidak dikirim langsung ke klien. Dto yang membentuk JSON,
//     dan service yang menentukan aturan.
//   - Setiap struct menyebut di kalimat pertama fungsi repository atau service yang mengisinya ("Dipakai oleh ...").
//
// Isinya:
//   - queue.go: Queue, yaitu baris tabel queues.
//   - queue_settings.go: QueueSettings dan SkipSettings, yaitu konfigurasi QR dan perilaku skip, beserta konstanta
//     nilainya.
//   - queue_status.go: QueueStatus, ServingEntry, dan UserTicket, yaitu gambaran antrian untuk pemegang tiket.
//   - ticket.go: Ticket, yaitu baris tabel tickets.
//   - ticket_note.go: TicketNote dan UpsertedNote, yaitu catatan owner per tiket.
//   - counter.go: Counter dan CounterWithServing, yaitu loket antrian.
//   - user.go: User, yaitu baris tabel users.
//   - profile.go: Profile dan Avatar, yaitu profil user yang login dan foto profilnya.
//   - display.go: QueueDisplay, DisplayQueueInfo, DisplayCounter, dan DisplaySnapshot, yaitu layar publik.
//   - history.go: HistoryQueue, HistoryQueueCursor, HistoryTicket, UserHistoryTicket, dan HistoryOutcome,
//     yaitu riwayat antrian yang sudah ditutup.
package model
