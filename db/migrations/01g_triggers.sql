-- ============================================================================
-- 01g_triggers.sql — Trigger
-- Isi      : fungsi dan trigger tickets_notify, tickets_email_outbox
-- Bergantung: 01b (counters), 01c (tickets), 01e (notification_outbox), 01a (users)
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

-- Dua trigger terpisah pada tickets, masing-masing satu fungsi. Isi fungsi tersimpan apa adanya di katalog
-- Postgres (termasuk komentar di dalamnya); jaga isinya sama dengan db/scripts/apply_schema.sql.

-- Sinyal untuk notify-service (LISTEN queue_events). Hanya memancarkan; pengiriman push terjadi di luar
-- transaksi Next/Skip. NOTIFY dikirim saat COMMIT, jadi transaksi yang dibatalkan tidak mengirim apa pun.
-- Event: requeued, skipped, called, recalled. Tanpa outbox: push yang terjadi saat notify-service mati hilang.
CREATE FUNCTION tickets_notify() RETURNS trigger AS $$
BEGIN
    -- requeued harus dicek paling awal: skip dengan fallback juga menaikkan skip_count, tetapi hasilnya nomor
    -- kembali menunggu, bukan skipped. Asal status bisa completed (Skip menurunkan nomor lebih dulu),
    -- currently_served (counter dihapus), atau skipped. reason membedakan dua sumber; counter dibawa di payload
    -- karena counter yang dihapus sudah tidak ada saat notify-service membaca.
    IF NEW.status = 'waiting' AND OLD.status IN ('currently_served', 'completed', 'skipped') THEN
        PERFORM pg_notify('queue_events', json_build_object(
            'kind', 'requeued', 'queue_id', NEW.queue_id, 'ticket_id', NEW.id,
            'reason', CASE WHEN NEW.skip_count > OLD.skip_count THEN 'skip' ELSE 'counter_closed' END,
            'counter', (SELECT c.name FROM counters c WHERE c.id = OLD.called_counter_id))::text);
    ELSIF NEW.skip_count > OLD.skip_count THEN
        PERFORM pg_notify('queue_events', json_build_object(
            'kind', 'skipped', 'queue_id', NEW.queue_id, 'ticket_id', NEW.id)::text);
    -- Transisi sementara: 'completed' dibiarkan selama masa transisi (lihat apply_schema.sql).
    ELSIF NEW.status IN ('completed', 'currently_served') AND NEW.called_at IS NOT NULL
       AND NEW.called_at IS DISTINCT FROM OLD.called_at THEN
        PERFORM pg_notify('queue_events', json_build_object(
            'kind', 'called', 'queue_id', NEW.queue_id, 'ticket_id', NEW.id)::text);
    ELSIF NEW.recalled_at IS NOT NULL AND NEW.recalled_at IS DISTINCT FROM OLD.recalled_at THEN
        PERFORM pg_notify('queue_events', json_build_object(
            'kind', 'recalled', 'queue_id', NEW.queue_id, 'ticket_id', NEW.id)::text);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tickets_notify_trg
    AFTER UPDATE OF status, called_at, recalled_at, skip_count ON tickets
    FOR EACH ROW EXECUTE FUNCTION tickets_notify();

-- Menulis notification_outbox untuk empat kejadian tiket: joined (INSERT), cancelled, removed (status deleted),
-- dan completed (hanya dari currently_served). Lalu membangunkan worker mail-service lewat pg_notify('mail_outbox').
CREATE FUNCTION tickets_email_outbox() RETURNS trigger AS $$
DECLARE
    k TEXT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        k := 'joined';
    ELSIF NEW.status = 'cancelled' AND OLD.status IS DISTINCT FROM 'cancelled' THEN
        k := 'cancelled';
    ELSIF NEW.status = 'deleted' AND OLD.status IS DISTINCT FROM 'deleted' THEN
        k := 'removed';
    ELSIF NEW.status = 'completed' AND OLD.status = 'currently_served' THEN
        k := 'completed';
    ELSE
        RETURN NULL;
    END IF;

    -- Sakelar pengguna dibaca saat kejadian: mematikan sakelar tidak membatalkan email lama,
    -- tetapi kejadian sesudahnya tidak masuk outbox.
    INSERT INTO notification_outbox (kind, ticket_id, user_id, payload)
    SELECT k, NEW.id, u.id, jsonb_build_object(
        'queue_id', q.id, 'queue_name', q.queue_name, 'prefix', q.ticket_prefix,
        'number', NEW.ticket_number, 'name', u.username)
    FROM users u JOIN queues q ON q.id = NEW.queue_id
    WHERE u.id = NEW.user_id AND u.email_notifications
    ON CONFLICT (kind, ticket_id) DO NOTHING;

    IF FOUND THEN
        PERFORM pg_notify('mail_outbox', '');
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tickets_email_outbox_trg
    AFTER INSERT OR UPDATE OF status ON tickets
    FOR EACH ROW EXECUTE FUNCTION tickets_email_outbox();
