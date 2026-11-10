// Package httpserver membungkus http.Server dengan timeout dasar dan shutdown yang rapi.
//
// Tidak ada WriteTimeout karena koneksi SSE (/queues/:id/event) berumur panjang; batas tulis untuk rute biasa
// dipegang nginx (proxy_read_timeout). Koneksi SSE ditutup lewat callback onShutdown (Hub.Shutdown), bukan lewat
// pembatalan konteks request, agar request biasa yang sedang berjalan (mis. query DB) tetap dituntaskan.

package httpserver