// Package apperror berisi error bersama yang dipakai lintas layer backend (repository, service, handler).
//
// Setiap error adalah variabel sentinel: service dan repository mengembalikannya, dan handler mencocokkannya
// dengan errors.Is untuk memilih status HTTP dan pesan yang dikirim ke client. Karena itu, error yang dibungkus
// dengan fmt.Errorf("...: %w", ...) tetap bisa dikenali.
//
// Error dikelompokkan menurut penyebabnya: data tidak ditemukan atau duplikat, input tidak valid, aturan keadaan
// antrian atau tiket, konflik dan batas kuota, langganan, token dan QR, serta kegagalan layanan eksternal.
package apperror
