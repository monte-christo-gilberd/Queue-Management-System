-- ============================================================================
-- 01j_push_devices.sql — Token push perangkat native (FCM)
-- Isi      : push_devices
-- Bergantung: 01a (users)
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

-- Token FCM per perangkat Android (aplikasi APK). Terpisah dari push_subscriptions karena bentuknya berbeda:
-- FCM memakai satu token, Web Push memakai endpoint dan dua kunci. Token yang sama dipindah ke akun yang terakhir
-- login (ON CONFLICT (token) di backend). Dibaca notify-service (role notify_ro), yang juga boleh menghapus token
-- yang sudah tidak berlaku (UNREGISTERED dari FCM).
CREATE TABLE push_devices (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform    TEXT NOT NULL CHECK (platform IN ('android')),
    token       TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
