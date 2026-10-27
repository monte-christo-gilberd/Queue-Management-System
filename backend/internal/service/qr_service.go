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

type QRService struct {
	repository   QueueRepository
	client       QRClient
	publicAppURL string
	publicQRURL  string
	notifier     EventNotifier
	dynamicTTL   time.Duration
}

func NewQRService(
	repository QueueRepository,
	client QRClient,
	publicAppURL string,
	publicQRURL string,
	notifiers ...EventNotifier,
) *QRService {
	svc := &QRService{
		repository:   repository,
		client:       client,
		publicAppURL: publicAppURL,
		publicQRURL:  publicQRURL,
		dynamicTTL:   120 * time.Second,
	}
	if len(notifiers) > 0 {
		svc.notifier = notifiers[0]
	}
	return svc
}

func (s *QRService) WithDynamicTTL(ttl time.Duration) *QRService {
	s.dynamicTTL = ttl
	return s
}

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
	if err := s.requireQueueOwner(ctx, queueID, requesterID); err != nil {
		return dto.QueueQRResponse{}, err
	}

	settings, err := s.getSettings(ctx, queueID)
	if err != nil {
		return dto.QueueQRResponse{}, err
	}

	qrType := settings.QRType
	if qrType == "" {
		qrType = qrTypeStatic
	}
	if qrType != qrTypeStatic && qrType != qrTypeDynamic {
		return dto.QueueQRResponse{}, apperror.ErrQRTypeInvalid
	}

	var token string
	var expiresAt *int64
	switch qrType {
	case qrTypeStatic:
		token = settings.QRCurrentCode.String
		if token == "" {
			issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeStatic, 0)
			if err != nil {
				return dto.QueueQRResponse{}, err
			}
			token = issued.Token
			if err := s.repository.UpdateQRCurrentCode(ctx, queueID, token); err != nil {
				return dto.QueueQRResponse{}, err
			}
		}
		expiresAt = nil
	case qrTypeDynamic:
		issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeDynamic, int(s.dynamicTTL.Seconds()))
		if err != nil {
			return dto.QueueQRResponse{}, err
		}
		token = issued.Token
		expiresAt = issued.ExpiresAt
	}

	// Tidak ada event di sini: GET hanya membaca. Event dikirim saat mode berubah
	// (SetQRType) atau kode diganti (ReplaceQR).
	return dto.NewQueueQRResponse(
		"queue_qr",
		queueID,
		qrType,
		s.joinURL(token),
		s.imageURL(token),
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
	if err := s.requireQueueOwner(ctx, queueID, requesterID); err != nil {
		return dto.QueueQRResponse{}, err
	}
	if qrType != qrTypeStatic && qrType != qrTypeDynamic {
		return dto.QueueQRResponse{}, apperror.ErrQRTypeInvalid
	}
	if err := s.repository.UpdateQRType(ctx, queueID, qrType); err != nil {
		return dto.QueueQRResponse{}, err
	}

	settings, err := s.getSettings(ctx, queueID)
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	qrType = settings.QRType
	if qrType == "" {
		qrType = qrTypeStatic
	}

	var token string
	var expiresAt *int64
	switch qrType {
	case qrTypeStatic:
		token = settings.QRCurrentCode.String
		if token == "" {
			issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeStatic, 0)
			if err != nil {
				return dto.QueueQRResponse{}, err
			}
			token = issued.Token
			if err := s.repository.UpdateQRCurrentCode(ctx, queueID, token); err != nil {
				return dto.QueueQRResponse{}, err
			}
		}
		expiresAt = nil
	case qrTypeDynamic:
		issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeDynamic, int(s.dynamicTTL.Seconds()))
		if err != nil {
			return dto.QueueQRResponse{}, err
		}
		token = issued.Token
		expiresAt = issued.ExpiresAt
	}

	if err := s.notifyQRUpdated(queueID, qrType, expiresAt); err != nil {
		return dto.QueueQRResponse{}, err
	}
	return dto.NewQueueQRResponse(
		"queue_qr",
		queueID,
		qrType,
		s.joinURL(token),
		s.imageURL(token),
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
	if err := s.requireQueueOwner(ctx, queueID, requesterID); err != nil {
		return dto.QueueQRResponse{}, err
	}

	settings, err := s.getSettings(ctx, queueID)
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	if settings.QRType != qrTypeStatic {
		return dto.QueueQRResponse{}, apperror.ErrQRReplaceNotAllowed
	}

	issued, err := s.client.IssueJoinToken(ctx, queueID, qrTypeStatic, 0)
	if err != nil {
		return dto.QueueQRResponse{}, err
	}
	if err := s.repository.UpdateQRCurrentCode(ctx, queueID, issued.Token); err != nil {
		return dto.QueueQRResponse{}, err
	}

	if err := s.notifyQRUpdated(queueID, qrTypeStatic, nil); err != nil {
		return dto.QueueQRResponse{}, err
	}
	return dto.NewQueueQRResponse(
		"queue_qr",
		queueID,
		qrTypeStatic,
		s.joinURL(issued.Token),
		s.imageURL(issued.Token),
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
	claims, err := s.client.VerifyJoinToken(ctx, token)
	if err != nil {
		return model.Queue{}, err
	}

	settings, err := s.getSettings(ctx, claims.QueueID)
	if err != nil {
		return model.Queue{}, apperror.ErrQRInvalid
	}
	if settings.QRType != claims.Type {
		return model.Queue{}, apperror.ErrQRInvalid
	}
	if claims.Type == qrTypeStatic {
		if !secureEqual(settings.QRCurrentCode.String, token) {
			return model.Queue{}, apperror.ErrQRInvalid
		}
	}

	queue, err := s.repository.GetQueueByID(ctx, claims.QueueID)
	if err != nil {
		return model.Queue{}, apperror.ErrQRInvalid
	}
	return queue, nil
}

func (s *QRService) requireQueueOwner(
	ctx context.Context,
	queueID int64,
	requesterID int64,
) error {
	queue, err := s.repository.GetQueueByID(ctx, queueID)
	if err != nil {
		return err
	}
	if queue.OwnerID != requesterID {
		return apperror.ErrNotFound
	}
	return nil
}

func (s *QRService) getSettings(
	ctx context.Context,
	queueID int64,
) (model.QueueSettings, error) {
	settings, err := s.repository.GetQueueSettings(ctx, queueID)
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return model.QueueSettings{}, err
	}
	if err := s.repository.CreateQueueSettingsIfMissing(ctx, queueID); err != nil {
		return model.QueueSettings{}, err
	}
	return s.repository.GetQueueSettings(ctx, queueID)
}

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

func (s *QRService) joinURL(token string) string {
	return strings.TrimRight(s.publicAppURL, "/") + "/join?t=" + token
}

func (s *QRService) imageURL(token string) string {
	return strings.TrimRight(s.publicQRURL, "/") + "/qr/join.png?token=" + token
}

func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
