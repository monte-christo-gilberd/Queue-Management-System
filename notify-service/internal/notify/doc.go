// Package notify berisi logika inti notify-service: untuk satu event, menentukan siapa yang diberi tahu
// dan teks apa yang dikirim, lalu mengirim push ke setiap langganan.
//
// Alur:
//   - Plan menghitung daftar Delivery (penerima dan isi pesan) dari event dan data tiket.
//   - Handle mengambil langganan setiap penerima, lalu memanggil Sender.Send. Langganan yang sudah tidak
//     berlaku (push.ErrGone) dihapus dari database. Kegagalan satu perangkat tidak menghentikan yang lain.
//
// Isinya:
//   - notify.go: Store dan Sender (kontrak), Service (NewService, Plan, Handle), dan Delivery.
//
// Package ini tidak membuka koneksi database atau HTTP. Dependensi itu dipasang dari luar lewat Store dan Sender.
package notify
