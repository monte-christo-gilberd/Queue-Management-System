package handler

import (
	"encoding/json"
	"net/http"
)

// respondJSON menulis respons JSON dengan status tertentu dan melarang cache.
func respondJSON(
	w http.ResponseWriter,
	status int,
	v any,
) {
	// Tandai respons sebagai JSON dan larang cache.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	// Tulis status, lalu encode nilai ke body. Galat encode diabaikan karena header sudah terkirim.
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// respondData menulis respons sukses dengan isi dibungkus di field "data".
func respondData(
	w http.ResponseWriter,
	status int,
	v any,
) {
	// Bungkus nilai ke field "data" lalu tulis sebagai JSON.
	respondJSON(w, status, map[string]any{"data": v})
}

// respondError menulis respons error dengan kode dan pesan dalam format yang seragam.
func respondError(
	w http.ResponseWriter,
	status int,
	code string,
	msg string,
) {
	// Bungkus kode dan pesan ke bentuk error standar lalu tulis sebagai JSON.
	respondJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": msg}})
}
