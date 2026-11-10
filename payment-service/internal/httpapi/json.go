package httpapi

import (
	"encoding/json"
	"net/http"
)

// writeJSON menulis respons JSON dengan status tertentu. Galat encode diabaikan karena header sudah terkirim.
func writeJSON(w http.ResponseWriter, status int, v any) {
	// Tandai respons sebagai JSON, lalu tulis status dan body.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError menulis respons error dengan kode dan pesan dalam format {"error": {"code", "message"}}.
func writeError(w http.ResponseWriter, status int, code, message string) {
	// Bungkus kode dan pesan ke bentuk error standar, lalu tulis sebagai JSON.
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
