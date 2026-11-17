package handler

import (
	"context"
	"net/http"
	"strings"

	"qms/internal/dto"

	"github.com/gin-gonic/gin"
)

// FCMDeviceStore adalah penyimpanan token FCM perangkat milik user (dipenuhi oleh repository).
type FCMDeviceStore interface {
	// UpsertDevice menyimpan token; bila token sudah ada, token dipindah ke user ini.
	UpsertDevice(ctx context.Context, userID int64, token, platform string) error
	// DeleteDevice menghapus token milik user.
	DeleteDevice(ctx context.Context, userID int64, token string) error
}

// FCMPushHandler menangani rute token FCM perangkat Android: daftar dan lepas. Padanan PushHandler (Web Push)
// untuk aplikasi Android. Backend hanya menyimpan token; pengiriman notifikasi dilakukan notify-service, yang
// membaca tabel push_devices dan memanggil FCM dengan kredensial service account.
type FCMPushHandler struct {
	// store menyimpan token perangkat.
	store FCMDeviceStore
}

// NewFCMPushHandler membuat FCMPushHandler dengan penyimpanan store.
func NewFCMPushHandler(store FCMDeviceStore) *FCMPushHandler {
	// isi handler dengan penyimpanan token
	return &FCMPushHandler{store: store}
}

// Register: POST /api/v1/push/fcm/register — simpan token FCM perangkat milik user yang login. 204 bila berhasil.
func (h *FCMPushHandler) Register(c *gin.Context) {
	// baca body JSON; token kosong atau platform tidak dikenal berarti request salah.
	// TrimSpace menolak token berisi spasi saja, yang lolos validasi "required" tetapi tidak mungkin sah.
	var req dto.FCMRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Token) == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// simpan token milik user yang sedang login (user_id dari JWT, bukan dari body); bila token sudah terdaftar
	// di akun lain, token dipindah ke akun ini. Gagal berarti error server
	if err := h.store.UpsertDevice(c.Request.Context(), c.GetInt64("user_id"), req.Token, req.Platform); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	// berhasil: 204 tanpa body (kontrak di docs/api-contract.md)
	c.Status(http.StatusNoContent)
}

// Unregister: POST /api/v1/push/fcm/unregister — hapus token FCM milik user yang login. 204 bila berhasil.
func (h *FCMPushHandler) Unregister(c *gin.Context) {
	// baca body JSON; token kosong berarti request salah
	var req dto.FCMUnregisterRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Token) == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// hapus token milik user yang sedang login saja; token milik akun lain tidak tersentuh (tetap 204, supaya
	// tidak membocorkan apakah token terdaftar di akun lain). Gagal berarti error server
	if err := h.store.DeleteDevice(c.Request.Context(), c.GetInt64("user_id"), req.Token); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	// berhasil: 204 tanpa body
	c.Status(http.StatusNoContent)
}
