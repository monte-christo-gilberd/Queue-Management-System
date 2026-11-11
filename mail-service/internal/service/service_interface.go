package service

import "context"

// MailServiceInterface: kontrak service kirim email langsung yang dipakai handler dan main. Dipenuhi *MailService.
type MailServiceInterface interface {
	// Send memeriksa penerima, mengisi templat, lalu mengirim email secara sinkron.
	Send(
		ctx context.Context,
		to string,
		template string,
		data map[string]any,
	) error
	// SendTest mengirim satu email uji ke alamat to (perintah send-test).
	SendTest(
		ctx context.Context,
		to string,
	) error
}

// OutboxServiceInterface: kontrak worker outbox yang dipakai main. Dipenuhi *OutboxService.
type OutboxServiceInterface interface {
	// Run menjalankan worker sampai ctx selesai.
	Run(
		ctx context.Context,
	)
	// Wake membangunkan worker sebelum polling berikutnya (dipanggil saat NOTIFY tiba).
	Wake()
}

// Pastikan implementasi memenuhi kontrak.
var (
	_ MailServiceInterface   = (*MailService)(nil)
	_ OutboxServiceInterface = (*OutboxService)(nil)
)
