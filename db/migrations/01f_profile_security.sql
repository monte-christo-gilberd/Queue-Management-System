-- ============================================================================
-- 01f_profile_security.sql — Profil dan keamanan akun
-- Isi      : user_avatars, user_action_tokens
-- Bergantung: 01a (users)
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

-- Foto profil: JPEG 512x512 hasil olahan server, satu per user. Disimpan di database supaya ikut backup.
-- Batas 150 KB dijaga CHECK. notify_ro sengaja tidak diberi hak apa pun di tabel ini.
CREATE TABLE user_avatars (
    user_id       BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    content_type  TEXT NOT NULL DEFAULT 'image/jpeg',
    data          BYTEA NOT NULL CHECK (octet_length(data) <= 153600),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Token sekali pakai untuk reset password dan verifikasi email. Yang disimpan adalah HASH token (SHA-256),
-- tidak pernah token mentahnya. 'email_change' disiapkan untuk verifikasi ganti email dan belum dipakai kode.
-- notify_ro dan mail_rw tidak diberi hak apa pun di tabel ini.
CREATE TABLE user_action_tokens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     TEXT NOT NULL CHECK (purpose IN ('password_reset', 'email_verify', 'email_change')),
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    request_ip  TEXT
);
