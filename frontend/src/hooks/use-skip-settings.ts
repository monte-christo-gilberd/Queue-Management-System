import { useCallback, useEffect, useState } from 'react';

import { describeQueueError } from '@/lib/queue-errors';
import { getSkipSettings, updateSkipSettings } from '@/lib/skip-settings';
import type { SkipSettings } from '@/types/api';

/** Muat dan simpan pengaturan Skip antrian (owner). */
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
