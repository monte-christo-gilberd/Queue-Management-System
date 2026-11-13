package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"
)

// QRService mengelola kode join antrian: kode statis yang dipakai terus sampai diganti owner, dan kode dinamis
// yang berganti setelah ada pengunjung yang berhasil join (atau setelah masa berlakunya habis).
// Kode 6 karakter dibuat di sini dan disimpan di queue_settings.qr_current_code; pengunjung cukup mengetik kodenya.

// Tipe QR dalam bentuk string, sama dengan model.QueueQRTypeStatic dan model.QueueQRTypeDynamic.
const (
	qrTypeStatic  = string(model.QueueQRTypeStatic)
	qrTypeDynamic = string(model.QueueQRTypeDynamic)
)

// maxCodeAttempts: berapa kali membuat kode acak bila bertabrakan dengan kode antrian lain.
const maxCodeAttempts = 5

// errJoinCodeExhausted: semua percobaan membuat kode unik gagal (sangat jarang, kode hanya ~1 miliar kombinasi).
var errJoinCodeExhausted = errors.New("could not generate unique join code")

// QRService: kode join antrian.
type QRService struct {
	// repository: penyimpanan antrian dan pengaturan QR (QueueRepository).
	repository QueueRepository
	// publicAppURL: alamat dasar aplikasi, dipakai membuat tautan join.
	publicAppURL string
	// notifier: mengirim event queue_qr_updated. Boleh nil.
	notifier EventNotifier
	// dynamicTTL: masa berlaku kode dinamis. Bawaan 120 detik.
	dynamicTTL time.Duration
	// now: sumber waktu; bisa diganti di tes.
	now func() time.Time
	// newCode: pembuat kode acak; bisa diganti di tes.
	newCode func() (string, error)
}

// NewQRService membuat QRService. publicAppURL dipakai untuk membuat tautan join. Notifier opsional
// (variadic): hanya notifiers[0] yang dipakai.
func NewQRService(
	repository QueueRepository,
	publicAppURL string,
	notifiers ...EventNotifier,
) *QRService {
	svc := &QRService{
		repository:   repository,
		publicAppURL: publicAppURL,
		dynamicTTL:   120 * time.Second,
		now:          time.Now,
		newCode:      model.NewJoinCode,
	}
	// Bila ada notifier yang diberikan, pakai yang pertama sebagai notifier service ini.
	if len(notifiers) > 0 {
		svc.notifier = notifiers[0]
	}
	return svc
}

// WithDynamicTTL mengubah masa berlaku kode dinamis dari nilai bawaan 120 detik.
func (s *QRService) WithDynamicTTL(ttl time.Duration) *QRService {
	s.dynamicTTL = ttl
	return s
}

// WithCodeGenerator mengganti pembuat kode acak. Dipakai tes agar kode yang dihasilkan bisa diprediksi.
func (s *QRService) WithCodeGenerator(gen func() (string, error)) *QRService {
	s.newCode = gen
	return s
}

// WithNotifier memasang notifier untuk event queue_qr_updated saat mode atau kode QR berubah.
func (s *QRService) WithNotifier(n EventNotifier) *QRService {
	s.notifier = n
	return s
}

// GetQR: operator (owner) membaca kode QR antrian. Kode statis tetap; kode dinamis tetap dipakai selama
// belum kedaluwarsa dan belum ada yang join.
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
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	qrType, err := activeQRType(settings)
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	code, expiresAt, err := s.activeCode(ctx, queueID, settings, qrType)
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Tidak ada event di sini: GET hanya membaca. Event dikirim saat mode berubah, kode diganti, atau kode dinamis berganti.
	return dto.NewQueueQRResponse("queue_qr", queueID, qrType, s.joinURL(code), expiresAt), nil
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
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	activeType, err := activeQRType(settings)
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	code, expiresAt, err := s.activeCode(ctx, queueID, settings, activeType)
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Kirim event queue_qr_updated agar klien tahu mode QR berubah.
	if err := s.notifyQRUpdated(queueID, activeType, expiresAt); err != nil {
		return dto.QueueQRResponse{}, err
	}
	return dto.NewQueueQRResponse("queue_qr", queueID, activeType, s.joinURL(code), expiresAt), nil
}

// ReplaceQR: operator mengganti kode statis dengan kode baru. Kode lama otomatis tidak cocok lagi karena
// kode aktif hanya satu (qr_current_code). Dinamis tidak bisa diganti karena sudah berganti sendiri.
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
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Penggantian hanya boleh untuk QR statis; bila tipenya bukan statis, tolak dengan ErrQRReplaceNotAllowed.
	if settings.QRType != qrTypeStatic {
		return dto.QueueQRResponse{}, apperror.ErrQRReplaceNotAllowed
	}
	code, err := s.newUniqueCode(ctx)
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Simpan kode baru sebagai kode statis aktif; kode lama otomatis tidak cocok lagi.
	if err := s.repository.UpdateQRCurrentCode(ctx, queueID, code); err != nil {
		return dto.QueueQRResponse{}, err
	}
	// Kirim event queue_qr_updated agar klien tahu kode QR sudah diganti.
	if err := s.notifyQRUpdated(queueID, qrTypeStatic, nil); err != nil {
		return dto.QueueQRResponse{}, err
	}
	return dto.NewQueueQRResponse("queue_qr", queueID, qrTypeStatic, s.joinURL(code), nil), nil
}

// ResolveJoinToken: mencari antrian dari kode join yang diketik atau dipindai. Kode dicari langsung di
// database; kode dinamis yang sudah kedaluwarsa ditolak dengan ErrQRExpired.
func (s *QRService) ResolveJoinToken(
	ctx context.Context,
	raw string,
) (model.Queue, error) {
	// Format kode salah (panjang atau karakter tidak termasuk alfabet kode): langsung tidak valid.
	code, ok := model.NormalizeJoinCode(raw)
	if !ok {
		return model.Queue{}, apperror.ErrQRInvalid
	}
	// Cari antrian yang sedang memakai kode ini. Kode lama yang sudah diganti tidak ada di sini.
	settings, err := s.repository.GetQueueSettingsByJoinCode(ctx, code)
	if err != nil {
		return model.Queue{}, apperror.ErrQRInvalid
	}
	// Kode dinamis wajib belum kedaluwarsa.
	if settings.QRType == qrTypeDynamic {
		expiresAt, ok := settings.QRCodeExpiresAt()
		if !ok || !s.now().Before(expiresAt) {
			return model.Queue{}, apperror.ErrQRExpired
		}
	}
	queue, err := s.repository.GetQueueByID(ctx, settings.QueueID)
	if err != nil {
		return model.Queue{}, apperror.ErrQRInvalid
	}
	return queue, nil
}

// AfterJoin: dipanggil setelah pengunjung berhasil join memakai kode raw. Bila antrian memakai QR dinamis dan
// kode itu masih yang aktif, kode diganti dengan yang baru. Kode statis tidak pernah berganti di sini.
func (s *QRService) AfterJoin(
	ctx context.Context,
	queueID int64,
	raw string,
) error {
	// Rapikan kode yang dipakai join. Kode yang tidak valid tidak mungkin aktif, jadi tidak ada yang perlu diganti.
	code, ok := model.NormalizeJoinCode(raw)
	if !ok {
		return nil
	}
	// Baca pengaturan terbaru antrian untuk memastikan tipe dan kode aktif masih sama.
	settings, err := s.repository.GetQueueSettings(ctx, queueID)
	if err != nil {
		return err
	}
	// Hanya kode dinamis yang aktif yang diganti; kode yang sudah diganti sebelumnya dibiarkan.
	if settings.QRType != qrTypeDynamic || settings.QRCurrentCode.String != code {
		return nil
	}
	// Siapkan kode pengganti dan masa berlaku barunya (sekarang + dynamicTTL).
	newCode, err := s.newUniqueCode(ctx)
	if err != nil {
		return err
	}
	expiresAt := s.now().Add(s.dynamicTTL)
	// Penggantian hanya terjadi bila kode aktif masih code; bila sudah berganti oleh join lain, tidak ada yang diubah.
	rotated, err := s.repository.RotateDynamicQRCode(ctx, queueID, code, newCode, expiresAt)
	if err != nil {
		return err
	}
	// Tidak ada yang berganti (kalah balapan dengan join lain): tidak perlu mengirim event.
	if !rotated {
		return nil
	}
	// Kode berganti: beri tahu layar owner agar menampilkan kode baru.
	unixExpiry := expiresAt.Unix()
	return s.notifyQRUpdated(queueID, qrTypeDynamic, &unixExpiry)
}

// activeQRType membaca tipe QR aktif dari pengaturan. Tipe kosong (data lama) dianggap statis.
func activeQRType(settings model.QueueSettings) (string, error) {
	qrType := settings.QRType
	if qrType == "" {
		qrType = qrTypeStatic
	}
	if qrType != qrTypeStatic && qrType != qrTypeDynamic {
		return "", apperror.ErrQRTypeInvalid
	}
	return qrType, nil
}

// activeCode mengembalikan kode join yang berlaku untuk tipe QR yang diberikan dan waktu kedaluwarsanya.
// Kode statis diterbitkan sekali dan dipakai terus. Kode dinamis dipakai selama belum kedaluwarsa; bila sudah
// lewat atau belum ada, diterbitkan kode baru.
func (s *QRService) activeCode(
	ctx context.Context,
	queueID int64,
	settings model.QueueSettings,
	qrType string,
) (string, *int64, error) {
	current := settings.QRCurrentCode.String
	// Kode tersimpan hanya dipakai bila berbentuk kode join 6 karakter. Token panjang dari versi lama
	// (data sebelum migrasi) dianggap tidak ada, sehingga diganti dengan kode baru.
	currentIsJoinCode := isJoinCode(current)
	if qrType == qrTypeStatic {
		// Kode statis yang masih berlaku: kode ada dan tidak membawa masa berlaku sisa dari mode dinamis.
		if currentIsJoinCode && !settings.QRExpiresAt.Valid {
			return current, nil, nil
		}
		code, err := s.newUniqueCode(ctx)
		if err != nil {
			return "", nil, err
		}
		if err := s.repository.UpdateQRCurrentCode(ctx, queueID, code); err != nil {
			return "", nil, err
		}
		return code, nil, nil
	}
	// Kode dinamis yang belum kedaluwarsa tetap dipakai.
	if expiresAt, ok := settings.QRCodeExpiresAt(); ok && currentIsJoinCode && s.now().Before(expiresAt) {
		unixExpiry := expiresAt.Unix()
		return current, &unixExpiry, nil
	}
	return s.issueDynamicCode(ctx, queueID)
}

// isJoinCode melaporkan apakah kode tersimpan berbentuk kode join yang valid.
func isJoinCode(code string) bool {
	_, ok := model.NormalizeJoinCode(code)
	return ok
}

// issueDynamicCode menerbitkan kode dinamis baru yang berlaku dynamicTTL dan menyimpannya sebagai kode aktif.
func (s *QRService) issueDynamicCode(
	ctx context.Context,
	queueID int64,
) (string, *int64, error) {
	code, err := s.newUniqueCode(ctx)
	if err != nil {
		return "", nil, err
	}
	expiresAt := s.now().Add(s.dynamicTTL)
	if err := s.repository.UpdateDynamicQRCode(ctx, queueID, code, expiresAt); err != nil {
		return "", nil, err
	}
	unixExpiry := expiresAt.Unix()
	return code, &unixExpiry, nil
}

// newUniqueCode membuat kode join acak yang belum dipakai antrian lain.
func (s *QRService) newUniqueCode(ctx context.Context) (string, error) {
	// Coba beberapa kali; setiap kode baru dicek apakah sudah dipakai antrian lain.
	for i := 0; i < maxCodeAttempts; i++ {
		// Buat kode acak. Gagal berarti sumber acak bermasalah, langsung kembalikan error.
		code, err := s.newCode()
		if err != nil {
			return "", err
		}
		// Cek apakah kode sudah dipakai. ErrNotFound berarti kode belum dipakai, jadi boleh dipakai.
		_, err = s.repository.GetQueueSettingsByJoinCode(ctx, code)
		if errors.Is(err, apperror.ErrNotFound) {
			return code, nil
		}
		// Error database selain "tidak ditemukan" dikembalikan.
		if err != nil {
			return "", err
		}
		// Kode sudah dipakai: lanjut ke percobaan berikutnya.
	}
	// Semua percobaan bertabrakan: kembalikan error khusus.
	return "", errJoinCodeExhausted
}

// requireQueueOwner memastikan requester adalah owner antrian; selain itu ErrNotFound.
func (s *QRService) requireQueueOwner(
	ctx context.Context,
	queueID int64,
	requesterID int64,
) error {
	queue, err := s.repository.GetQueueByID(ctx, queueID)
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
	settings, err := s.repository.GetQueueSettings(ctx, queueID)
	if err == nil {
		return settings, nil
	}
	// Error selain "tidak ditemukan": kembalikan error aslinya.
	if !errors.Is(err, apperror.ErrNotFound) {
		return model.QueueSettings{}, err
	}
	// Baris pengaturan belum ada (antrian lama): buat baris default lebih dulu.
	if err := s.repository.CreateQueueSettingsIfMissing(ctx, queueID); err != nil {
		return model.QueueSettings{}, err
	}
	return s.repository.GetQueueSettings(ctx, queueID)
}

// notifyQRUpdated mengirim event queue_qr_updated berisi tipe QR dan waktu kedaluwarsa (kosong untuk statis).
func (s *QRService) notifyQRUpdated(
	queueID int64,
	qrType string,
	expiresAt *int64,
) error {
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

// joinURL membuat tautan join yang dipindai pengunjung, bentuknya: <publicAppURL>/join?t=<kode>.
func (s *QRService) joinURL(code string) string {
	return strings.TrimRight(s.publicAppURL, "/") + "/join?t=" + code
}
