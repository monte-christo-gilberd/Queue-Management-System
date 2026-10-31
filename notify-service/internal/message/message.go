// Package message berisi tipe data bersama dan format teks notifikasi.
package message

import "fmt"

// Label: satu-satunya tempat format nomor tiket untuk teks notifikasi. Prefix kosong = "A".
func Label(prefix string, number int) string {
	if prefix == "" {
		prefix = "A"
	}
	return fmt.Sprintf("%s-%03d", prefix, number)
}

type Ticket struct {
	ID        int64
	QueueID   int64
	Number    int
	UserID    int64
	Status    string
	QueueName string
	Prefix    string
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
