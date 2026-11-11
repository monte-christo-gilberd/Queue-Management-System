package apperror

import "errors"

var (
	// 1. Templat: nama templat tidak dikenal, atau data tidak melengkapi templat.
	ErrUnknownTemplate = errors.New("unknown email template")
	ErrBadData         = errors.New("template data is incomplete")

	// 2. Input tidak valid: data dari pemanggil tidak memenuhi aturan.
	ErrInvalidRecipient = errors.New("invalid recipient")

	// 3. Pengiriman: batas bersamaan terlampaui, atau SMTP gagal (ErrPermanent bila tidak ada gunanya diulang).
	ErrBusy       = errors.New("too many emails in flight, retry later")
	ErrSendFailed = errors.New("email could not be sent")
	ErrPermanent  = errors.New("permanent smtp failure")
)
