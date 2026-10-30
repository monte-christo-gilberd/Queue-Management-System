import { CameraView, useCameraPermissions } from 'expo-camera';
import { router } from 'expo-router';
import { useCallback, useRef, useState } from 'react';
import { Pressable, Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';
import { AppColors } from '@/constants/app-theme';
import { describeQueueError } from '@/lib/queue-errors';
import { joinByToken } from '@/lib/qr';
import { parseJoinToken } from '@/lib/qr-scan';

type JoinQueueModalsProps = {
  visible: boolean;
  onClose: () => void;
};

/**
 * Popup "Masuk ke Antrian": memindai QR antrian dengan kamera perangkat. Begitu QR terbaca,
 * `POST /join` dipanggil otomatis tanpa klik. Halaman `/join?t=` tidak melakukan join, jadi
 * ini satu-satunya jalur join dari UI.
 */
export function JoinQueueModals({ visible, onClose }: JoinQueueModalsProps) {
  return (
    <ModalCard visible={visible} onRequestClose={onClose}>
      {/* Dipasang hanya saat terbuka: state scan dan kamera ter-reset tiap dibuka. */}
      {visible ? <ScannerBody onClose={onClose} /> : null}
    </ModalCard>
  );
}

function ScannerBody({ onClose }: { onClose: () => void }) {
  const [permission, requestPermission] = useCameraPermissions();
  const [message, setMessage] = useState<string>();
  const [busy, setBusy] = useState(false);
  // Kunci sinkron: onBarcodeScanned bisa terpanggil berkali-kali sebelum state sempat berubah.
  const lockRef = useRef(false);

  const handleScanned = useCallback(
    async ({ data }: { data: string }) => {
      if (lockRef.current) return;
      lockRef.current = true;
      const token = parseJoinToken(data);
      if (!token) {
        setMessage('QR ini bukan QR antrian.');
        setTimeout(() => (lockRef.current = false), 1500);
        return;
      }
      setBusy(true);
      setMessage(undefined);
      try {
        const ticket = await joinByToken(token);
        onClose();
        router.replace({ pathname: '/ticket/[queueId]', params: { queueId: String(ticket.queue_id) } });
      } catch (e) {
        setMessage(describeQueueError(e));
        setBusy(false);
        setTimeout(() => (lockRef.current = false), 1500);
      }
    },
    [onClose],
  );

  const granted = permission?.granted === true;

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

      <Pressable onPress={onClose} className="items-center py-1" accessibilityRole="button">
        <Text className="text-[13px] font-bold" style={{ color: AppColors.textMuted }}>
          Tutup
        </Text>
      </Pressable>
    </>
  );
}
