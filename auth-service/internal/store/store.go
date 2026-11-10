package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // mendaftarkan driver "pgx" untuk sqlx
	"github.com/jmoiron/sqlx"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/model"
)

// Connect membuka koneksi dan memastikan database terjangkau (Ping, 5 detik).
func Connect(dsn string) (*sqlx.DB, error) {
	// buka koneksi dengan driver pgx (didaftarkan oleh import kosong stdlib di atas)
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, err
	}
	// batas 5 detik untuk ping; melepas timer saat fungsi selesai (bukan menutup koneksi)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// pastikan database benar-benar bisa dihubungi
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

// UserStore mengakses tabel users.
type UserStore struct {
	// db: koneksi PostgreSQL untuk query di luar transaksi.
	db *sqlx.DB
}

// NewUserStore membuat UserStore.
func NewUserStore(db *sqlx.DB) *UserStore { return &UserStore{db: db} }

// Create menyimpan user baru. ErrDuplicate bila melanggar UNIQUE (kode pgconn 23505).
func (r *UserStore) Create(ctx context.Context, email, username, passwordHash string) (model.User, error) {
	const query = `
		INSERT INTO users(email, username, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, username, password_hash, google_id, created_at`
	var user model.User
	// jalankan INSERT dan ambil kembali kolom user yang baru dibuat (RETURNING)
	err := r.db.GetContext(ctx, &user, query, email, username, passwordHash)
	var pgErr *pgconn.PgError
	// kode 23505 = pelanggaran UNIQUE (email sudah dipakai)
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return model.User{}, apperror.ErrDuplicate
	}
	// galat lain dibungkus dengan konteks
	if err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// FindByLogin mengembalikan user yang email-nya ATAU username-nya sama dengan identifier (maks 10; email didahulukan).
func (r *UserStore) FindByLogin(ctx context.Context, identifier string) ([]model.User, error) {
	const query = `
		SELECT id, email, username, password_hash, google_id, created_at
		FROM users
		WHERE email = $1 OR username = $1
		ORDER BY (email = $1) DESC, id ASC
		LIMIT 10`
	var users []model.User
	// username tidak unik, jadi hasilnya bisa lebih dari satu baris
	if err := r.db.SelectContext(ctx, &users, query, identifier); err != nil {
		return nil, fmt.Errorf("find user by login: %w", err)
	}
	return users, nil
}

// FindByEmail: id, email, username untuk alur reset. ErrNotFound bila tidak ada.
func (r *UserStore) FindByEmail(ctx context.Context, email string) (model.User, error) {
	var u model.User
	// email dicocokkan persis (peka huruf besar/kecil)
	err := r.db.GetContext(ctx, &u, `SELECT id, email, username FROM users WHERE email = $1`, email)
	// tidak ada baris: ErrNotFound agar pemanggil tidak membocorkan email yang terdaftar
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user for reset: %w", err)
	}
	return u, nil
}

// FindForVerification: id, email, username, email_verified_at. ErrNotFound bila tidak ada.
func (r *UserStore) FindForVerification(ctx context.Context, userID int64) (model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `SELECT id, email, username, email_verified_at FROM users WHERE id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user for verification: %w", err)
	}
	return u, nil
}

// PasswordChangedAt: kapan password terakhir diganti; nil bila belum pernah. ErrNotFound bila user tidak ada.
func (r *UserStore) PasswordChangedAt(ctx context.Context, userID int64) (*time.Time, error) {
	// NullTime karena kolomnya NULL bila password belum pernah diganti
	var t sql.NullTime
	err := r.db.GetContext(ctx, &t, `SELECT password_changed_at FROM users WHERE id = $1`, userID)
	// user tidak ada: ErrNotFound
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get password_changed_at: %w", err)
	}
	// NULL: password belum pernah diganti
	if !t.Valid {
		return nil, nil
	}
	return &t.Time, nil
}

// accountCols: kolom akun lengkap yang dibaca untuk login Google.
const accountCols = `id, email, username, password_hash, google_id, created_at`

// FindByGoogleID: akun yang tertaut ke akun Google ini. ErrNotFound bila tidak ada.
func (r *UserStore) FindByGoogleID(ctx context.Context, googleID string) (model.User, error) {
	return r.findOne(ctx, "find user by google id", `SELECT `+accountCols+` FROM users WHERE google_id = $1`, googleID)
}

// FindAccountByEmail: akun dengan email ini tanpa membedakan huruf besar atau kecil (registrasi lama tidak
// menormalkan email, jadi "Budi@x.com" dan "budi@x.com" harus dikenali sebagai akun yang sama). Bila ada lebih dari
// satu, yang paling lama dipakai. ErrNotFound bila tidak ada.
func (r *UserStore) FindAccountByEmail(ctx context.Context, email string) (model.User, error) {
	return r.findOne(ctx, "find user by email", `SELECT `+accountCols+` FROM users WHERE lower(email) = lower($1) ORDER BY id LIMIT 1`, email)
}

// FindByID: akun lengkap dengan id ini. ErrNotFound bila tidak ada.
func (r *UserStore) FindByID(ctx context.Context, id int64) (model.User, error) {
	return r.findOne(ctx, "find user by id", `SELECT `+accountCols+` FROM users WHERE id = $1`, id)
}

// findOne menjalankan query yang mengembalikan satu akun; tidak ada baris menjadi ErrNotFound.
func (r *UserStore) findOne(ctx context.Context, what, query string, args ...any) (model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, query, args...)
	// tidak ada baris: ErrNotFound
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	// galat lain dibungkus dengan konteks
	if err != nil {
		return model.User{}, fmt.Errorf("%s: %w", what, err)
	}
	return u, nil
}

// CreateGoogle membuat akun tanpa password. email_verified_at langsung diisi karena Google sudah menjamin emailnya.
// ErrDuplicate bila email atau google_id melanggar UNIQUE (kode pgconn 23505).
func (r *UserStore) CreateGoogle(ctx context.Context, email, username, googleID string) (model.User, error) {
	const query = `
		INSERT INTO users(email, username, google_id, email_verified_at)
		VALUES ($1, $2, $3, now())
		RETURNING ` + accountCols
	var u model.User
	err := r.db.GetContext(ctx, &u, query, email, username, googleID)
	var pgErr *pgconn.PgError
	// kode 23505 = pelanggaran UNIQUE
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return model.User{}, apperror.ErrDuplicate
	}
	if err != nil {
		return model.User{}, fmt.Errorf("create google user: %w", err)
	}
	return u, nil
}

// LinkGoogle menautkan google_id ke akun yang belum tertaut dan menandai emailnya terverifikasi (Google menjamin
// email yang sama). ErrDuplicate bila akun sudah tertaut, atau google_id sudah dipakai akun lain.
func (r *UserStore) LinkGoogle(ctx context.Context, userID int64, googleID string) error {
	// google_id IS NULL di WHERE: dua permintaan serentak, hanya satu yang berhasil menautkan.
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET google_id = $2, email_verified_at = COALESCE(email_verified_at, now()) WHERE id = $1 AND google_id IS NULL`,
		userID, googleID)
	var pgErr *pgconn.PgError
	// google_id itu sudah dipakai akun lain
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperror.ErrDuplicate
	}
	if err != nil {
		return fmt.Errorf("link google: %w", err)
	}
	// tidak ada baris yang berubah: akun sudah tertaut ke akun Google lain
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return apperror.ErrDuplicate
	}
	return nil
}

// TokenStore mengakses tabel user_action_tokens (hanya hash token yang disimpan).
type TokenStore struct {
	// db: koneksi PostgreSQL; transaksi dimulai dari sini.
	db *sqlx.DB
}

// NewTokenStore membuat TokenStore.
func NewTokenStore(db *sqlx.DB) *TokenStore { return &TokenStore{db: db} }

// CountRecentTokens menghitung token purpose ini milik user yang dibuat setelah since (batas kirim per jam).
func (r *TokenStore) CountRecentTokens(ctx context.Context, userID int64, purpose string, since time.Time) (int, error) {
	var n int
	// hitung token user dengan purpose ini yang dibuat setelah since
	err := r.db.GetContext(ctx, &n,
		`SELECT count(*) FROM user_action_tokens WHERE user_id = $1 AND purpose = $2 AND created_at > $3`,
		userID, purpose, since)
	if err != nil {
		return 0, fmt.Errorf("count recent tokens: %w", err)
	}
	return n, nil
}

// CreateToken mencabut token lama (purpose sama) lalu menyimpan token baru, dalam satu transaksi.
func (r *TokenStore) CreateToken(ctx context.Context, userID int64, purpose, tokenHash string, expiresAt time.Time, requestIP string) error {
	// satu transaksi agar pencabutan token lama dan penyisipan token baru berjalan atomik
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// rollback otomatis bila keluar sebelum Commit; setelah Commit berhasil tidak berpengaruh
	defer tx.Rollback()
	// tandai token lama (purpose sama) yang belum dipakai sebagai terpakai, agar hanya token baru yang berlaku
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`,
		userID, purpose); err != nil {
		return fmt.Errorf("revoke old tokens: %w", err)
	}
	// simpan token baru (hash, purpose, kedaluwarsa, IP peminta)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_action_tokens (user_id, purpose, token_hash, expires_at, request_ip) VALUES ($1, $2, $3, $4, $5)`,
		userID, purpose, tokenHash, expiresAt, requestIP); err != nil {
		return fmt.Errorf("insert token: %w", err)
	}
	return tx.Commit()
}

// ResetPassword memakai token reset: token ditandai terpakai, password diganti, token reset lain dicabut (atomik).
// Token tidak sah, sudah dipakai, atau kedaluwarsa: ErrInvalidResetToken.
func (r *TokenStore) ResetPassword(ctx context.Context, tokenHash, passwordHash string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	// Satu UPDATE bersyarat: dua permintaan serentak dengan token yang sama, hanya satu yang mendapat baris.
	var userID int64
	err = tx.GetContext(ctx, &userID, `
		UPDATE user_action_tokens SET used_at = now()
		WHERE token_hash = $1 AND purpose = 'password_reset' AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash)
	// tidak ada baris yang berubah: token salah, sudah dipakai, atau kedaluwarsa
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	// ganti password dan catat waktunya; token login yang terbit sebelumnya jadi dicabut
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = $2, password_changed_at = now() WHERE id = $1`, userID, passwordHash); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	// cabut semua token reset lain milik user agar tautan lama tidak bisa dipakai lagi
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = 'password_reset' AND used_at IS NULL`,
		userID); err != nil {
		return fmt.Errorf("revoke other tokens: %w", err)
	}
	return tx.Commit()
}

// VerifyEmail memakai token verifikasi: token ditandai terpakai, email_verified_at diisi, token lain dicabut (atomik).
// Token tidak sah, sudah dipakai, atau kedaluwarsa: ErrInvalidResetToken.
func (r *TokenStore) VerifyEmail(ctx context.Context, tokenHash string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	err = tx.GetContext(ctx, &userID, `
		UPDATE user_action_tokens SET used_at = now()
		WHERE token_hash = $1 AND purpose = 'email_verify' AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash)
	// token salah, sudah dipakai, atau kedaluwarsa
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	// COALESCE: akun yang sudah terverifikasi tidak dimajukan waktunya.
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET email_verified_at = COALESCE(email_verified_at, now()) WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("set email verified: %w", err)
	}
	// cabut token verifikasi lain milik user
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = 'email_verify' AND used_at IS NULL`,
		userID); err != nil {
		return fmt.Errorf("revoke other tokens: %w", err)
	}
	return tx.Commit()
}
