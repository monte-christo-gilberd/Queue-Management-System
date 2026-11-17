/**
 * join: halaman gabung antrian dari tautan QR `/join?t=<kode>`.
 *
 * Rute        : /join?t=<kode> (isi QR antrian; dibuka dari kamera bawaan HP atau tautan yang dibagikan).
 * Akses       : butuh login (AuthGate mode required). Tamu diarahkan ke login lalu kembali ke /join?t=<kode> yang sama.
 * Dipakai oleh: tautan dari QR antrian (join_url dari GET /queues/:id/qr).
 * Bergantung  : AuthGate, AppButton, lib/qr (previewJoin, joinByToken), lib/queue-errors (describeQueueError).
 * Alur         : buka -> GET /join/preview?t= (info antrian) -> user menekan Gabung -> POST /join {t} ->
 *                router.replace ke /ticket/[queueId].
 * Catatan     : Popup "Masuk ke Antrian" di dashboard (JoinQueueModals) tetap jalur join lewat kamera aplikasi;
 *                halaman ini untuk QR yang dipindai di luar aplikasi.
 */
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Text, View } from 'react-native';

import { AuthGate } from '@/components/auth/AuthGate';
import { AppButton } from '@/components/ui/AppButton';
import { previewJoin, joinByToken } from '@/lib/qr';
import { describeQueueError } from '@/lib/queue-errors';
import type { PublicQueue } from '@/types/api';

/** MAX_TOKEN_LENGTH: batas panjang kode di URL, sama dengan lib/qr-scan.ts. */
const MAX_TOKEN_LENGTH = 2048;

/**
 * Halaman gabung antrian. Kode di `?t` dibawa ke login lewat `nextPath`, karena usePathname() tidak membawa query.
 */
export default function JoinRoute() {
  // `t` bisa kosong, berupa array (parameter berulang), atau sangat panjang; hanya string bukan-kosong dan wajar yang dipakai.
  const { t } = useLocalSearchParams<{ t?: string }>();
  const token = typeof t === 'string' ? t.trim() : '';
  const valid = token !== '' && token.length <= MAX_TOKEN_LENGTH;
  // Kode tidak valid tetap dirender (JoinContent menampilkan pesan), tapi tidak dibawa ke login sebagai `next`.
  return (
    <AuthGate mode="required" nextPath={valid ? `/join?t=${encodeURIComponent(token)}` : '/join'}>
      <JoinContent token={valid ? token : null} />
    </AuthGate>
  );
}

/** Isi halaman setelah login: muat info antrian dari kode, lalu tombol Gabung. */
function JoinContent({ token }: { token: string | null }) {
  // Info antrian dari preview; null selama dimuat atau bila gagal.
  const [queue, setQueue] = useState<PublicQueue | null>(null);
  // Tanpa kode tidak ada yang dimuat, jadi tidak perlu spinner.
  const [loading, setLoading] = useState(token !== null);
  // Galat preview atau join, sudah dalam bahasa ramah. Kode kosong langsung diberi pesan sejak awal.
  const [error, setError] = useState<string | undefined>(
    token === null ? 'Tautan antrian tidak lengkap. Pindai ulang QR dari penyelenggara.' : undefined,
  );
  // true selama POST /join berjalan; mengunci tombol agar tidak terkirim dua kali.
  const [joining, setJoining] = useState(false);

  // Muat info antrian sekali per kode. Guard `active` mencegah setState setelah unmount.
  useEffect(() => {
    if (token === null) return;
    let active = true;
    previewJoin(token).then(
      (q) => {
        if (!active) return;
        setQueue(q);
        setLoading(false);
      },
      (e) => {
        if (!active) return;
        // QR_INVALID, QR_EXPIRED, dan galat jaringan dipetakan ke pesan; kode tidak dikenal jatuh ke pesan umum.
        setError(describeQueueError(e));
        setLoading(false);
      },
    );
    return () => {
      active = false;
    };
  }, [token]);

  /** POST /join lalu buka halaman tiket. Bila sudah punya tiket aktif, backend mengembalikan tiket yang sama. */
  async function handleJoin() {
    if (token === null) return;
    setJoining(true);
    setError(undefined);
    try {
      const ticket = await joinByToken(token);
      router.replace({ pathname: '/ticket/[queueId]', params: { queueId: String(ticket.queue_id) } });
    } catch (e) {
      // Kode dinamis bisa kedaluwarsa di antara preview dan join; pesannya meminta pindai ulang.
      setError(describeQueueError(e));
      setJoining(false);
    }
  }

  // Antrian yang sudah ditutup tetap ditampilkan, tapi tombol Gabung disembunyikan (join akan ditolak QUEUE_CLOSED).
  const closed = queue?.status === 'closed';

  return (
    <View className="flex-1 items-center justify-center gap-4 bg-background p-6">
      <Text className="text-center text-[24px] font-extrabold text-foreground">Masuk ke Antrian</Text>

      {/* Spinner selama preview dimuat; setelah itu salah satu: kartu antrian, atau pesan galat. */}
      {loading ? <ActivityIndicator /> : null}

      {queue ? (
        <View className="w-full max-w-[420px] items-center gap-2 rounded-card bg-surface p-5">
          <Text className="text-center text-[20px] font-extrabold text-foreground">{queue.queue_name}</Text>
          {queue.queue_desc ? (
            <Text className="text-center text-[14px] text-muted">{queue.queue_desc}</Text>
          ) : null}
          {closed ? (
            <Text className="text-center text-[13px] font-bold text-live">Antrian ini sudah ditutup.</Text>
          ) : null}
        </View>
      ) : null}

      {error ? <Text className="text-center text-[13px] text-live">{error}</Text> : null}

      {/* Gabung hanya bila info antrian berhasil dimuat dan antrian masih buka. Ke Dashboard selalu ada sebagai jalan keluar. */}
      <View className="w-full max-w-[420px] gap-3">
        {queue && !closed ? (
          <AppButton label="Gabung Antrian" onPress={() => void handleJoin()} loading={joining} />
        ) : null}
        <AppButton label="Ke Dashboard" variant="outline" onPress={() => router.replace('/dashboard')} disabled={joining} />
      </View>
    </View>
  );
}
