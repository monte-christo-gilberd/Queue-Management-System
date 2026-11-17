/**
 * JoinQueueModals: popup "Masuk ke Antrian" untuk customer, memindai QR dengan kamera atau menempel tautan antrian.
 *
 * Dipakai oleh : app/(user)/dashboard.tsx, components/layout/ResponsiveScreenLayout.tsx
 * Bergantung   : expo-camera, expo-router, lib/qr (joinByToken), lib/qr-scan (parseJoinToken),
 *                lib/queue-errors (describeQueueError), components/ui/ModalCard, components/ui/AppButton
 * Alur         : QR terbaca / tautan ditempel -> joinByToken -> popup tutup -> router.replace ke /ticket/[queueId]
 * Catatan      : Jalur join lewat kamera aplikasi. QR yang dipindai di luar aplikasi membuka halaman /join?t= (app/join.tsx).
 */
import { CameraView, useCameraPermissions } from 'expo-camera';
import { router } from 'expo-router';
import { useCallback, useRef, useState } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';
import { AppColors } from '@/constants/app-theme';
import { describeQueueError } from '@/lib/queue-errors';
import { joinByToken } from '@/lib/qr';
import { parseJoinToken } from '@/lib/qr-scan';

type JoinQueueModalsProps = {
  /** Popup tampil bila true. */
  visible: boolean;
  /** Dipanggil saat popup ditutup manual (Tutup, latar, back) atau setelah join berhasil. */
  onClose: () => void;
};

/**
 * Popup "Masuk ke Antrian": memindai QR antrian dengan kamera perangkat, atau menempel tautan antrian
 * (`<origin>/join?t=...`) di form. Begitu QR terbaca, `POST /join` dipanggil otomatis tanpa klik; tautan
 * yang ditempel diproses saat tombol Masuk (atau Enter) ditekan. QR yang dipindai di luar aplikasi membuka
 * halaman `/join?t=` (app/join.tsx), yang menampilkan info antrian lalu join setelah dikonfirmasi.
 */
export function JoinQueueModals({ visible, onClose }: JoinQueueModalsProps) {
  return (
    <ModalCard visible={visible} onRequestClose={onClose}>
      {/* Dipasang hanya saat terbuka: state scan dan kamera ter-reset tiap dibuka. */}
      {visible ? <ScannerBody onClose={onClose} /> : null}
    </ModalCard>
  );
}

/** Isi popup: kamera (atau tombol izin), kotak masukkan token, dan status join. */
function ScannerBody({ onClose }: { onClose: () => void }) {
  const [permission, requestPermission] = useCameraPermissions();
  const [message, setMessage] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [link, setLink] = useState('');
  // Kunci sinkron: onBarcodeScanned bisa terpanggil berkali-kali sebelum state sempat berubah.
  const lockRef = useRef(false);

  /**
   * Join dari isi QR atau tautan yang ditempel. `retryMs` = jeda sebelum boleh mencoba lagi setelah gagal
   * (scan butuh jeda supaya kamera tidak mengulang QR yang sama terus-menerus; form langsung boleh dicoba lagi).
   */
  const attempt = useCallback(
    async (raw: string, invalidMessage: string, retryMs: number) => {
      if (lockRef.current) return;
      lockRef.current = true;
      // Buka kunci setelah jeda retryMs; bila 0, langsung dibuka.
      const unlock = () => {
        if (retryMs > 0) setTimeout(() => (lockRef.current = false), retryMs);
        else lockRef.current = false;
      };
      const token = parseJoinToken(raw);
      if (!token) {
        setMessage(invalidMessage);
        unlock();
        return;
      }
      setBusy(true);
      setMessage(undefined);
      try {
        // Bila berhasil, kunci tidak dibuka lagi: popup tutup dan tiket langsung dibuka.
        const ticket = await joinByToken(token);
        onClose();
        router.replace({ pathname: '/ticket/[queueId]', params: { queueId: String(ticket.queue_id) } });
      } catch (e) {
        // Galat ditampilkan sebagai pesan di popup, lalu kunci dibuka agar bisa dicoba lagi.
        setMessage(describeQueueError(e));
        setBusy(false);
        unlock();
      }
    },
    [onClose],
  );

  // Dipanggil kamera untuk setiap QR yang terbaca; scan dengan jeda 1.5 detik setelah gagal.
  const handleScanned = useCallback(
    ({ data }: { data: string }) => void attempt(data, 'QR ini bukan QR antrian.', 1500),
    [attempt],
  );

  // Token atau tautan yang dimasukkan tidak perlu jeda: form boleh langsung dicoba lagi.
  const submitLink = () => void attempt(link, 'Token ini tidak valid.', 0);

  const granted = permission?.granted === true;

  // Tanpa izin: tampil tombol minta izin. Bila izin ditolak permanen (canAskAgain false), tombol disembunyikan.
  return (
    <>
      <Text className="text-[24px] font-extrabold text-foreground">Masuk ke Antrian</Text>

      <View className="h-64 w-full items-center justify-center overflow-hidden rounded-card bg-dark">
        {granted ? (
          <CameraView
            style={{ width: '100%', height: '100%' }}
            facing="back"
            barcodeScannerSettings={{ barcodeTypes: ['qr'] }}
            onBarcodeScanned={busy ? undefined : handleScanned}
          />
        ) : (
          <View className="items-center gap-3 px-4">
            <Text className="text-center text-[13px] text-white">
              {permission?.canAskAgain === false
                ? 'Izin kamera ditolak. Aktifkan izin kamera di pengaturan browser atau perangkat.'
                : 'Izinkan akses kamera untuk memindai QR antrian.'}
            </Text>
            {permission?.canAskAgain !== false ? (
              <AppButton label="Izinkan Kamera" variant="outline" onPress={() => void requestPermission()} />
            ) : null}
          </View>
        )}
      </View>

      <Text className="text-center text-[12px] text-muted">
        {busy ? 'Bergabung ke antrian...' : message ?? 'Arahkan kamera ke QR code antrian.'}
      </Text>

      {/* Tanpa kamera: masukkan token 6 karakter dari penyelenggara (tautan juga diterima). */}
      <Text className="text-center text-[12px] text-muted">atau masukkan token antrian (6 karakter)</Text>
      <TextInput
        value={link}
        onChangeText={setLink}
        editable={!busy}
        placeholder="Token 6 karakter"
        placeholderTextColor={AppColors.placeholder}
        autoCapitalize="characters"
        autoCorrect={false}
        maxLength={2048}
        returnKeyType="go"
        onSubmitEditing={submitLink}
        className="h-[48px] rounded-card bg-surface px-3.5 text-[14px] text-foreground"
      />
      <AppButton label={busy ? 'Bergabung...' : 'Masuk'} variant="accent" disabled={busy || link.trim() === ''} onPress={submitLink} />

      <Pressable onPress={onClose} className="items-center py-1" accessibilityRole="button">
        <Text className="text-[13px] font-bold" style={{ color: AppColors.textMuted }}>
          Tutup
        </Text>
      </Pressable>
    </>
  );
}
