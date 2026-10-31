// Package message berisi tipe data bersama dan format teks notifikasi.
package message

import "fmt"

// Label: satu-satunya tempat format nomor tiket untuk teks notifikasi (fitur prefix nanti hanya mengubah ini).
func Label(number int) string {
	return fmt.Sprintf("A-%03d", number)
}

type Ticket struct {
	ID        int64
	QueueID   int64
	Number    int
	UserID    int64
	Status    string
	QueueName string
}

type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}
