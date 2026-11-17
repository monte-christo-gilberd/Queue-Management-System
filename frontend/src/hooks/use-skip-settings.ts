/**
 * use-skip-settings: pengaturan Skip satu antrian untuk owner (muat dan simpan).
 *
 * Dipakai oleh : src/app/(user)/queues/[queueId]/index.tsx.
 * Bergantung   : lib/skip-settings (getSkipSettings, updateSkipSettings), lib/queue-errors (describeQueueError), types/api (SkipSettings).
 * Alur         : queueId -> getSkipSettings -> settings. save -> updateSkipSettings -> settings baru.
 * Catatan      : Pengaturan dimuat sekali per queueId. Galat muat dan simpan masuk ke `error`.
 */

import { useCallback, useEffect, useState } from 'react';

import { describeQueueError } from '@/lib/queue-errors';
import { getSkipSettings, updateSkipSettings } from '@/lib/skip-settings';
import type { SkipSettings } from '@/types/api';

/**
 * Muat dan simpan pengaturan Skip antrian (owner).
 *
 * Argumen: `queueId` adalah id antrian.
 *
 * Mengembalikan objek dengan field:
 * - `settings`: pengaturan Skip saat ini. null sebelum dimuat.
 * - `saving`: true selama request simpan berjalan.
 * - `error`: pesan galat muat atau simpan. undefined bila tidak ada.
 * - `saved`: true setelah simpan berhasil, dan kembali false saat simpan berikutnya dimulai.
 * - `save(input)`: menyimpan pengaturan baru ke server.
 */
export function useSkipSettings(queueId: number) {
  const [settings, setSettings] = useState<SkipSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    let active = true;
    void getSkipSettings(queueId).then(
      (s) => {
        if (active) setSettings(s);
      },
      (e: unknown) => {
        if (active) setError(describeQueueError(e));
      },
    );
    return () => {
      active = false;
    };
  }, [queueId]);

  const save = useCallback(
    async (input: SkipSettings) => {
      // Keadaan `saved` dan `error` direset dulu supaya pesan dari simpan sebelumnya tidak tertinggal.
      setSaving(true);
      setError(undefined);
      setSaved(false);
      try {
        setSettings(await updateSkipSettings(queueId, input));
        setSaved(true);
      } catch (e) {
        setError(describeQueueError(e));
      } finally {
        setSaving(false);
      }
    },
    [queueId],
  );

  return { settings, saving, error, saved, save };
}
