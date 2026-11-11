package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/model"
)

// UserRepository mengakses tabel users.
type UserRepository struct {
	// db: koneksi PostgreSQL untuk query di luar transaksi.
	db *sqlx.DB
}

// NewUserRepository membuat UserRepository.
func NewUserRepository(db *sqlx.DB) *UserRepository { return &UserRepository{db: db} }

// Create menyimpan user baru. ErrDuplicate bila melanggar UNIQUE (kode pgconn 23505).
func (r *UserRepository) Create(
	ctx context.Context,
	email,
	username,
	passwordHash string,
) (model.User, error) {
	// Query ini menyisipkan user baru dan mengembalikan kolom akun yang dibuat database (id, created_at).
	const query = `
		INSERT INTO users(email, username, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, username, password_hash, google_id, created_at`
	// Menampung baris hasil RETURNING.
	var user model.User
	// jalankan INSERT dan ambil kembali kolom user yang baru dibuat (RETURNING)
	err := r.db.GetContext(ctx, &user, query, email, username, passwordHash)
	// Menampung detail galat PostgreSQL bila ada.
	var pgErr *pgconn.PgError
	// kode 23505 = pelanggaran UNIQUE (email sudah dipakai)
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// Terjemahkan ke ErrDuplicate agar service tidak bergantung pada kode driver.
		return model.User{}, apperror.ErrDuplicate
	}
	// galat lain dibungkus dengan konteks
	if err != nil {
		// Galat database dibungkus dengan konteks.
		return model.User{}, fmt.Errorf("create user: %w", err)
	}
	// Kembalikan user yang baru dibuat.
	return user, nil
}

// FindByLogin mengembalikan user yang email-nya ATAU username-nya sama dengan identifier (maks 10; email didahulukan).
func (r *UserRepository) FindByLogin(
	ctx context.Context,
	identifier string,
) ([]model.User, error) {
	// Query ini mencari user berdasarkan email atau username; email yang cocok diurutkan lebih dulu, dibatasi 10 baris.
	const query = `
		SELECT id, email, username, password_hash, google_id, created_at
		FROM users
		WHERE email = $1 OR username = $1
		ORDER BY (email = $1) DESC, id ASC
		LIMIT 10`
	// Menampung semua baris yang cocok.
	var users []model.User
	// username tidak unik, jadi hasilnya bisa lebih dari satu baris
	if err := r.db.SelectContext(ctx, &users, query, identifier); err != nil {
		// Galat database dibungkus dengan konteks.
		return nil, fmt.Errorf("find user by login: %w", err)
	}
	// Kembalikan kandidat; pemilihan akun yang benar dilakukan service.
	return users, nil
}

// FindByEmail: id, email, username untuk alur reset. ErrNotFound bila tidak ada.
func (r *UserRepository) FindByEmail(
	ctx context.Context,
	email string,
) (model.User, error) {
	// Menampung baris hasil query.
	var u model.User
	// email dicocokkan persis (peka huruf besar/kecil)
	err := r.db.GetContext(ctx, &u, `SELECT id, email, username FROM users WHERE email = $1`, email)
	// tidak ada baris: ErrNotFound agar pemanggil tidak membocorkan email yang terdaftar
	if errors.Is(err, sql.ErrNoRows) {
		// Tidak ada baris dipetakan ke ErrNotFound.
		return model.User{}, apperror.ErrNotFound
	}
	// Galat database lain dibungkus dengan konteks.
	if err != nil {
		// Galat database dibungkus dengan konteks.
		return model.User{}, fmt.Errorf("find user for reset: %w", err)
	}
	// Kembalikan data user untuk alur reset.
	return u, nil
}

// FindForVerification: id, email, username, email_verified_at. ErrNotFound bila tidak ada.
func (r *UserRepository) FindForVerification(
	ctx context.Context,
	userID int64,
) (model.User, error) {
	// Menampung baris hasil query.
	var u model.User
	// Query ini mengambil id, email, username, dan email_verified_at berdasarkan id.
	err := r.db.GetContext(ctx, &u, `SELECT id, email, username, email_verified_at FROM users WHERE id = $1`, userID)
	// Tidak ada baris: user tidak ada.
	if errors.Is(err, sql.ErrNoRows) {
		// Dipetakan ke ErrNotFound.
		return model.User{}, apperror.ErrNotFound
	}
	// Galat database lain dibungkus dengan konteks.
	if err != nil {
		return model.User{}, fmt.Errorf("find user for verification: %w", err)
	}
	// Kembalikan data user untuk pemeriksaan verifikasi.
	return u, nil
}

// PasswordChangedAt: kapan password terakhir diganti; nil bila belum pernah. ErrNotFound bila user tidak ada.
func (r *UserRepository) PasswordChangedAt(
	ctx context.Context,
	userID int64,
) (*time.Time, error) {
	// NullTime karena kolomnya NULL bila password belum pernah diganti
	var t sql.NullTime
	// Query ini mengambil kolom password_changed_at milik user.
	err := r.db.GetContext(ctx, &t, `SELECT password_changed_at FROM users WHERE id = $1`, userID)
	// user tidak ada: ErrNotFound
	if errors.Is(err, sql.ErrNoRows) {
		// Dipetakan ke ErrNotFound.
		return nil, apperror.ErrNotFound
	}
	// Galat database lain dibungkus dengan konteks.
	if err != nil {
		return nil, fmt.Errorf("get password_changed_at: %w", err)
	}
	// NULL: password belum pernah diganti
	if !t.Valid {
		// Kembalikan nil: tidak ada waktu penggantian.
		return nil, nil
	}
	// Kembalikan penunjuk ke waktu penggantian terakhir.
	return &t.Time, nil
}

// accountCols: kolom akun lengkap yang dibaca untuk login Google.
const accountCols = `id, email, username, password_hash, google_id, created_at`

// FindByGoogleID: akun yang tertaut ke akun Google ini. ErrNotFound bila tidak ada.
func (r *UserRepository) FindByGoogleID(
	ctx context.Context,
	googleID string,
) (model.User, error) {
	// Cari akun yang google_id-nya sama persis.
	return r.findOne(ctx, "find user by google id", `SELECT `+accountCols+` FROM users WHERE google_id = $1`, googleID)
}

// FindAccountByEmail: akun dengan email ini tanpa membedakan huruf besar atau kecil (registrasi lama tidak
// menormalkan email, jadi "Budi@x.com" dan "budi@x.com" harus dikenali sebagai akun yang sama). Bila ada lebih dari
// satu, yang paling lama dipakai. ErrNotFound bila tidak ada.
func (r *UserRepository) FindAccountByEmail(
	ctx context.Context,
	email string,
) (model.User, error) {
	// Query membandingkan lower(email) dengan lower(input); ORDER BY id LIMIT 1 memilih akun tertua bila ada beberapa.
	return r.findOne(ctx, "find user by email", `SELECT `+accountCols+` FROM users WHERE lower(email) = lower($1) ORDER BY id LIMIT 1`, email)
}

// FindByID: akun lengkap dengan id ini. ErrNotFound bila tidak ada.
func (r *UserRepository) FindByID(
	ctx context.Context,
	id int64,
) (model.User, error) {
	// Cari akun berdasarkan primary key.
	return r.findOne(ctx, "find user by id", `SELECT `+accountCols+` FROM users WHERE id = $1`, id)
}

// findOne menjalankan query yang mengembalikan satu akun; tidak ada baris menjadi ErrNotFound.
func (r *UserRepository) findOne(
	ctx context.Context,
	what,
	query string,
	args ...any,
) (model.User, error) {
	// Menampung baris hasil query.
	var u model.User
	// Jalankan query yang dikirim pemanggil dan isi u.
	err := r.db.GetContext(ctx, &u, query, args...)
	// tidak ada baris: ErrNotFound
	if errors.Is(err, sql.ErrNoRows) {
		// Dipetakan ke ErrNotFound.
		return model.User{}, apperror.ErrNotFound
	}
	// galat lain dibungkus dengan konteks
	if err != nil {
		// what menjelaskan operasi yang gagal untuk log.
		return model.User{}, fmt.Errorf("%s: %w", what, err)
	}
	// Kembalikan akun yang ditemukan.
	return u, nil
}

// CreateGoogle membuat akun tanpa password. email_verified_at langsung diisi karena Google sudah menjamin emailnya.
// ErrDuplicate bila email atau google_id melanggar UNIQUE (kode pgconn 23505).
func (r *UserRepository) CreateGoogle(
	ctx context.Context,
	email,
	username,
	googleID string,
) (model.User, error) {
	// Query ini menyisipkan akun Google (tanpa password) dengan email_verified_at = sekarang, lalu mengembalikan kolom akun.
	const query = `
		INSERT INTO users(email, username, google_id, email_verified_at)
		VALUES ($1, $2, $3, now())
		RETURNING ` + accountCols
	// Menampung baris hasil RETURNING.
	var u model.User
	// Jalankan INSERT dan ambil akun yang baru dibuat.
	err := r.db.GetContext(ctx, &u, query, email, username, googleID)
	// Menampung detail galat PostgreSQL bila ada.
	var pgErr *pgconn.PgError
	// kode 23505 = pelanggaran UNIQUE
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// Dipetakan ke ErrDuplicate: email atau google_id sudah dipakai.
		return model.User{}, apperror.ErrDuplicate
	}
	// Galat database lain dibungkus dengan konteks.
	if err != nil {
		return model.User{}, fmt.Errorf("create google user: %w", err)
	}
	// Kembalikan akun yang baru dibuat.
	return u, nil
}

// LinkGoogle menautkan google_id ke akun yang belum tertaut dan menandai emailnya terverifikasi (Google menjamin
// email yang sama). ErrDuplicate bila akun sudah tertaut, atau google_id sudah dipakai akun lain.
func (r *UserRepository) LinkGoogle(
	ctx context.Context,
	userID int64,
	googleID string,
) error {
	// google_id IS NULL di WHERE: dua permintaan serentak, hanya satu yang berhasil menautkan.
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET google_id = $2, email_verified_at = COALESCE(email_verified_at, now()) WHERE id = $1 AND google_id IS NULL`,
		userID, googleID)
	// Menampung detail galat PostgreSQL bila ada.
	var pgErr *pgconn.PgError
	// google_id itu sudah dipakai akun lain
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// Dipetakan ke ErrDuplicate.
		return apperror.ErrDuplicate
	}
	// Galat database lain dibungkus dengan konteks.
	if err != nil {
		// Galat database dibungkus dengan konteks.
		return fmt.Errorf("link google: %w", err)
	}
	// tidak ada baris yang berubah: akun sudah tertaut ke akun Google lain
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		// Akun sudah tertaut ke akun Google lain.
		return apperror.ErrDuplicate
	}
	// Penautan berhasil.
	return nil
}
