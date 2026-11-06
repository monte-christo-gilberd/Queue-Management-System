-- ============================================================================
-- 01h_role_grants.sql — Hak role
-- Isi      : blok GRANT bersyarat untuk role mail_rw
-- Bergantung: 01a (users), 01e (notification_outbox)
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

-- Role per service dibuat oleh 03..05_*_role.sh SETELAH file ini, dan skrip itu memberi hak sendiri. Blok ini
-- hanya berlaku bila role mail_rw SUDAH ada saat file ini dijalankan (mis. database dengan role yang tersisa),
-- jadi pada init database kosong ia tidak melakukan apa-apa. Daftar hak mail_rw sama dengan 04_mail_role.sh.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mail_rw') THEN
        GRANT USAGE ON SCHEMA public TO mail_rw;
        GRANT SELECT ON notification_outbox TO mail_rw;
        GRANT UPDATE (sent_at, attempts, last_error, next_attempt_at, failed_at) ON notification_outbox TO mail_rw;
        GRANT SELECT (id, email, username, email_notifications) ON users TO mail_rw;
    END IF;
END $$;
