// Package apperror berisi galat sentinel yang dikembalikan repository dan service, lalu dipetakan ke kode galat
// HTTP oleh package handler dan middleware.
//
// Isinya:
//   - apperror.go: ErrNotFound, ErrDuplicate, ErrWeakPassword, ErrTooManyVerifyEmails, ErrInvalidResetToken,
//     ErrUnauthenticated, serta galat login Google (ErrGoogleInvalidToken, ErrGoogleEmailNotVerified,
//     ErrGoogleUnavailable, ErrGoogleLinkRequired, ErrInvalidLinkToken).
package apperror
