package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"

	qrclient "qms/internal/qrclient"
)

// QRService mengelola QR antrian: pembuatan token statis/dinamis, penggantian
// kode statis, dan verifikasi token untuk join. Orchestrasi-nya ada di sini;
// detail token ada di qrclient, detail penyimpanan di repository.

// Tipe QR dalam bentuk string, sama dengan model.QueueQRTypeStatic dan model.QueueQRTypeDynamic.
const (
	qrTypeStatic  = string(model.QueueQRTypeStatic)
	qrTypeDynamic = string(model.QueueQRTypeDynamic)
)

// QRClient: kontrak ke qr-service (dipenuhi *qrclient.Client).
type QRClient interface {
	IssueJoinToken(
		ctx context.Context,
		queueID int64,
		typ string,
		ttlSeconds int,
	) (qrclient.JoinToken, error)
	VerifyJoinToken(
		ctx context.Context,
		token string,
	) (qrclient.JoinTokenClaims, error)
}

// QRService: QR join antrian. QR statis memakai satu kode yang disimpan di queue_settings dan bisa diganti
// owner. QR dinamis membuat token baru yang berlaku dynamicTTL (bawaan 120 detik).
type QRService struct {
	// repository: penyimpanan antrian dan pengaturan QR (QueueRepository).
	repository QueueRepository
	// client: layanan qr-service untuk membuat dan memverifikasi token join (QRClient).
	client QRClient
	// publicAppURL: alamat dasar aplikasi, dipakai membuat tautan join.
	publicAppURL string
	// notifier: mengirim event queue_qr_updated. Boleh nil.
	notifier EventNotifier
	// dynamicTTL: masa berlaku QR dinamis. Bawaan 120 detik.
	dynamicTTL time.Duration
}

// NewQRService membuat QRService. publicAppURL dipakai untuk membuat tautan join. Notifier opsional
// (variadic): hanya notifiers[0] yang dipakai.
func NewQRService(
	repository QueueRepository,
	client QRClient,
	publicAppURL string,
	notifiers ...EventNotifier,
) *QRService {
	svc := &QRService{
		repository:   repository,
		client:       client,
		publicAppURL: publicAppURL,
		dynamicTTL:   120 * time.Second,
	}
	// Bila ada notifier yang diberikan, pakai yang pertama sebagai notifier service ini.
	if len(notifiers) > 0 {
		svc.notifier = notifiers[0]
	}
	return svc
}

// WithDynamicTTL mengubah masa berlaku QR dinamis dari nilai bawaan 120 detik.
func (s *QRService) WithDynamicTTL(ttl time.Duration) *QRService {
	s.dynamicTTL = ttl
	return s
}

// WithNotifier memasang notifier untuk event queue_qr_updated saat mode atau kode QR berubah.
func (s *QRService) WithNotifier(n EventNotifier) *QRService {
	s.notifier = n
	return s
}

// GetQR: operator (owner) membaca status QR antrian. Untuk statis: token dipakai
// (dibuat sekali dan disimpan); untuk dinamis: token baru dipakai setiap kali dipanggil.
func (s *QRService) GetQR(
	ctx context.Context,
	queueID int64,
	requesterID int64,
) (dto.QueueQRResponse, error) {
	// Pastikan requester adalah owner antrian; bila bukan, kembalikan error.
	if err := s.requireQueueOwner(ctx, queueID, requesterID); err != nil {
		return dto.QueueQRResponse{}, err
	}

	// Ambil pengaturan QR antrian dari queue_settings (dibuat bila belum ada).
	settings, err := s.getSettings(ctx, queueID)
	// Gagal membaca pengaturan: kembalikan error.
	if err != nil {
		return dto.QueueQRResponse{}, err
	}

	qrType := settings.QRType
	// Tipe QR kosong (data lama): anggap statis sebagai nilai bawaan.
	if qrType == "" {
		qrType = qrTypeStatic
	}
	// Tipe QR tidak dikenal (bukan statis maupun dinamis): tolak dengan ErrQRTypeInvalid.
	if qrType != qrTypeStatic && qrType != qrTypeDynamic {
		return dto.QueueQRResponse{}, apperror.ErrQRTypeInvalid
	}

	var token string
	var expiresAt *int64
	// Tentukan token dan masa berlaku sesuai tipe QR.
	switch qrType {
	// Tipe statis: pakai kode yang tersimpan, dan buat kode baru hanya bila belum ada.
	case qrTypeStatic:
		token = settings.QRCurrentCode.String
		// Belum ada kode statis tersimpan: terbitkan token baru lewat qr-service.
		if token == "" {
			issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeStatic, 0)
			// Gagal menerbitkan token: kembalikan error.
			if err != nil {
				return dto.QueueQRResponse{}, err
			}
			token = issued.Token
			// Simpan token baru sebagai kode statis aktif agar dipakai di pemanggilan berikutnya.
			if err := s.repository.UpdateQRCurrentCode(ctx, queueID, token); err != nil {
				return dto.QueueQRResponse{}, err
			}
		}
		expiresAt = nil
	// Tipe dinamis: terbitkan token baru setiap kali dipanggil, berlaku selama dynamicTTL.
	case qrTypeDynamic:
		issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeDynamic, int(s.dynamicTTL.Seconds()))
		// Gagal menerbitkan token dinamis: kembalikan error.
		if err != nil {
			return dto.QueueQRResponse{}, err
		}
		token = issued.Token
		expiresAt = issued.ExpiresAt
	}

	// Tidak ada event di sini: GET hanya membaca. Event dikirim saat mode berubah
	// (SetQRType) atau kode diganti (ReplaceQR).
	// Bungkus tipe, ID antrian, dan tautan join (dengan token) menjadi respons QR.
	return dto.NewQueueQRResponse(
		"queue_qr",
		queueID,
		qrType,
		s.joinURL(token),
		expiresAt,
	), nil
}

// SetQRType: operator mengubah mode statis <-> dinamis.
func (s *QRService) SetQRType(
	ctx context.Context,
	queueID int64,
	requesterID int64,
	qrType string,
) (dto.QueueQRResponse, error) {
	// Pastikan requester adalah owner antrian; bila bukan, kembalikan error.
	if err := s.requireQueueOwner(ctx, queueID, requesterID); err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Tipe yang diminta bukan statis maupun dinamis: tolak dengan ErrQRTypeInvalid.
	if qrType != qrTypeStatic && qrType != qrTypeDynamic {
		return dto.QueueQRResponse{}, apperror.ErrQRTypeInvalid
	}
	// Simpan tipe QR baru ke queue_settings; gagal berarti kembalikan error.
	if err := s.repository.UpdateQRType(ctx, queueID, qrType); err != nil {
		return dto.QueueQRResponse{}, err
	}

	// Baca ulang pengaturan QR setelah diubah, untuk memastikan data terbaru yang dipakai.
	settings, err := s.getSettings(ctx, queueID)
	// Gagal membaca pengaturan: kembalikan error.
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	qrType = settings.QRType
	// Tipe QR kosong (data lama): anggap statis sebagai nilai bawaan.
	if qrType == "" {
		qrType = qrTypeStatic
	}

	var token string
	var expiresAt *int64
	// Terbitkan atau pakai token sesuai tipe QR yang sekarang aktif.
	switch qrType {
	// Tipe statis: pakai kode yang tersimpan, dan buat kode baru hanya bila belum ada.
	case qrTypeStatic:
		token = settings.QRCurrentCode.String
		// Belum ada kode statis tersimpan: terbitkan token baru lewat qr-service.
		if token == "" {
			issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeStatic, 0)
			// Gagal menerbitkan token: kembalikan error.
			if err != nil {
				return dto.QueueQRResponse{}, err
			}
			token = issued.Token
			// Simpan token baru sebagai kode statis aktif.
			if err := s.repository.UpdateQRCurrentCode(ctx, queueID, token); err != nil {
				return dto.QueueQRResponse{}, err
			}
		}
		expiresAt = nil
	// Tipe dinamis: terbitkan token baru yang berlaku selama dynamicTTL.
	case qrTypeDynamic:
		issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeDynamic, int(s.dynamicTTL.Seconds()))
		// Gagal menerbitkan token dinamis: kembalikan error.
		if err != nil {
			return dto.QueueQRResponse{}, err
		}
		token = issued.Token
		expiresAt = issued.ExpiresAt
	}

	// Kirim event queue_qr_updated agar klien tahu mode QR berubah; gagal berarti kembalikan error.
	if err := s.notifyQRUpdated(queueID, qrType, expiresAt); err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Bungkus tipe baru dan tautan join menjadi respons QR.
	return dto.NewQueueQRResponse(
		"queue_qr",
		queueID,
		qrType,
		s.joinURL(token),
		expiresAt,
	), nil
}

// ReplaceQR: operator mengganti kode statis dengan kode baru. Token lama otomatis
// tidak cocok lagi (backend membandingkan dengan qr_current_code). Dinamis tidak
// bisa diganti karena sudah berganti sendiri lewat masa berlaku.
func (s *QRService) ReplaceQR(
	ctx context.Context,
	queueID int64,
	requesterID int64,
) (dto.QueueQRResponse, error) {
	// Pastikan requester adalah owner antrian; bila bukan, kembalikan error.
	if err := s.requireQueueOwner(ctx, queueID, requesterID); err != nil {
		return dto.QueueQRResponse{}, err
	}

	// Ambil pengaturan QR antrian untuk mengecek tipe QR yang aktif.
	settings, err := s.getSettings(ctx, queueID)
	// Gagal membaca pengaturan: kembalikan error.
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Penggantian hanya boleh untuk QR statis; bila tipenya bukan statis, tolak dengan ErrQRReplaceNotAllowed.
	if settings.QRType != qrTypeStatic {
		return dto.QueueQRResponse{}, apperror.ErrQRReplaceNotAllowed
	}

	// Terbitkan kode statis baru lewat qr-service.
	issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeStatic, 0)
	// Gagal menerbitkan kode baru: kembalikan error.
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Simpan kode baru sebagai kode statis aktif; token lama otomatis tidak cocok lagi.
	if err := s.repository.UpdateQRCurrentCode(ctx, queueID, issued.Token); err != nil {
		return dto.QueueQRResponse{}, err
	}

	// Kirim event queue_qr_updated agar klien tahu kode QR sudah diganti.
	if err := s.notifyQRUpdated(queueID, qrTypeStatic, nil); err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Bungkus kode baru dan tautan join menjadi respons QR (statis, tanpa masa berlaku).
	return dto.NewQueueQRResponse(
		"queue_qr",
		queueID,
		qrTypeStatic,
		s.joinURL(issued.Token),
		nil,
	), nil
}

// ResolveJoinToken: verifikasi token join (via qr-service) lalu bantu load antrian.
// Join dan preview selalu memverifikasi ulang; jangan percaya nilai `qid` dari klien.
// Token statis wajib cocok dengan kode aktif di queue_settings (penggantian = paksa pakai kode baru).
func (s *QRService) ResolveJoinToken(
	ctx context.Context,
	token string,
) (model.Queue, error) {
	// Verifikasi token ke qr-service untuk mendapatkan klaim (ID antrian dan tipe token).
	claims, err := s.client.VerifyJoinToken(ctx, token)
	// Token tidak valid atau gagal diverifikasi: kembalikan error.
	if err != nil {
		return model.Queue{}, err
	}

	// Ambil pengaturan QR antrian yang disebut di dalam token.
	settings, err := s.getSettings(ctx, claims.QueueID)
	// Pengaturan tidak bisa dibaca: anggap token tidak valid.
	if err != nil {
		return model.Queue{}, apperror.ErrQRInvalid
	}
	// Tipe QR di pengaturan berbeda dengan tipe di token: token sudah tidak berlaku.
	if settings.QRType != claims.Type {
		return model.Queue{}, apperror.ErrQRInvalid
	}
	// Token statis harus sama dengan kode aktif; perbandingan dilakukan dengan waktu tetap.
	if claims.Type == qrTypeStatic {
		// Kode tidak cocok dengan kode aktif (misalnya sudah diganti): tolak dengan ErrQRInvalid.
		if !secureEqual(settings.QRCurrentCode.String, token) {
			return model.Queue{}, apperror.ErrQRInvalid
		}
	}

	// Ambil data antrian yang disebut di dalam token.
	queue, err := s.repository.GetQueueByID(ctx, claims.QueueID)
	// Antrian tidak bisa dibaca: anggap token tidak valid.
	if err != nil {
		return model.Queue{}, apperror.ErrQRInvalid
	}
	return queue, nil
}

// requireQueueOwner memastikan requester adalah owner antrian; selain itu ErrNotFound.
func (s *QRService) requireQueueOwner(
	ctx context.Context,
	queueID int64,
	requesterID int64,
) error {
	// Ambil antrian dari repository untuk dicek pemiliknya.
	queue, err := s.repository.GetQueueByID(ctx, queueID)
	// Antrian tidak ditemukan atau gagal dibaca: kembalikan error.
	if err != nil {
		return err
	}
	// Requester bukan owner: sembunyikan keberadaan antrian dengan ErrNotFound.
	if queue.OwnerID != requesterID {
		return apperror.ErrNotFound
	}
	return nil
}

// getSettings membaca queue_settings. Bila baris belum ada (antrian lama), baris dibuat dulu lalu dibaca lagi.
func (s *QRService) getSettings(
	ctx context.Context,
	queueID int64,
) (model.QueueSettings, error) {
	// Coba baca pengaturan antrian dari repository.
	settings, err := s.repository.GetQueueSettings(ctx, queueID)
	// Pengaturan berhasil dibaca: langsung kembalikan.
	if err == nil {
		return settings, nil
	}
	// Error selain "tidak ditemukan": bukan karena baris belum ada, jadi kembalikan error aslinya.
	if !errors.Is(err, apperror.ErrNotFound) {
		return model.QueueSettings{}, err
	}
	// Baris pengaturan belum ada (antrian lama): buat baris default lebih dulu.
	if err := s.repository.CreateQueueSettingsIfMissing(ctx, queueID); err != nil {
		return model.QueueSettings{}, err
	}
	// Baca ulang pengaturan yang baru dibuat.
	return s.repository.GetQueueSettings(ctx, queueID)
}

// notifyQRUpdated mengirim event queue_qr_updated berisi tipe QR dan waktu kedaluwarsa (kosong untuk statis).
func (s *QRService) notifyQRUpdated(
	queueID int64,
	qrType string,
	expiresAt *int64,
) error {
	// Kirim event queue_qr_updated berisi ID antrian, tipe QR, dan waktu kedaluwarsa.
	notifyEvent(
		s.notifier,
		queueID,
		EventQueueQRUpdated,
		map[string]interface{}{
			"queue_id":   queueID,
			"type":       qrType,
			"action":     "queue_qr",
			"expires_at": expiresAt,
		},
	)
	return nil
}

// joinURL membuat tautan join yang dipindai pengunjung, bentuknya: <publicAppURL>/join?t=<token>.
func (s *QRService) joinURL(token string) string {
	// Buang garis miring di akhir URL dasar, lalu tambahkan jalur /join dan token.
	return strings.TrimRight(s.publicAppURL, "/") + "/join?t=" + token
}

// secureEqual membandingkan dua string dalam waktu yang tetap, supaya selisih waktu tidak membocorkan isi kode.
func secureEqual(a, b string) bool {
	// Bandingkan kedua string sebagai byte dengan waktu konstan; hasil 1 berarti sama.
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
