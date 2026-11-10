/**
 * ModalCard: kerangka popup (modal) bersama dengan latar gelap dan toast viewport di dalamnya.
 *
 * Dipakai oleh : components/CounterBar.tsx, components/MyHistoryList.tsx, components/QueueQRCard.tsx,
 *                components/CustomerQueueScreen.tsx, components/CreateQueueModal.tsx,
 *                components/ui/AppendNoteModal.tsx, components/ui/JoinQueueModals.tsx,
 *                components/ui/ConfirmModal.tsx, components/ui/MyTicketModal.tsx,
 *                components/ui/InsertTicketModal.tsx
 * Bergantung   : components/ui/Toast (useToastModalOpen, ToastViewport)
 * Catatan      : Tap latar gelap memanggil onRequestClose. Isi kartu hanya menelan tap, tidak menutup modal.
 */
import { Modal, Pressable, View } from 'react-native';

import { ToastViewport, useToastModalOpen } from '@/components/ui/Toast';

type ModalCardProps = {
  /** true = modal tampil (dengan animasi fade); false = modal disembunyikan. */
  visible: boolean;
  /** Dipanggil saat latar gelap ditekan atau tombol back perangkat. Penutupan dikerjakan induk lewat visible. */
  onRequestClose: () => void;
  /** Isi kartu putih di tengah. */
  children: React.ReactNode;
};

/** Modal putih di tengah layar dengan latar gelap, seperti "Append Note". */
export function ModalCard({ visible, onRequestClose, children }: ModalCardProps) {
  // Selama modal terbuka, toast galat tampil lewat viewport di dalam modal (di atas latar gelapnya).
  const toastId = useToastModalOpen(visible);
  // Kartu diberi onPress kosong agar tap di dalam kartu tidak sampai ke latar dan menutup modal.
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onRequestClose}>
      <View className="flex-1">
        <Pressable className="flex-1 justify-center bg-modalBackdrop px-[30px]" onPress={onRequestClose}>
          <Pressable className="w-full max-w-[480px] self-center rounded-3xl bg-background p-5" onPress={() => {}}>
            <View className="gap-3.5 rounded-card">{children}</View>
          </Pressable>
        </Pressable>
        <ToastViewport modalId={toastId} />
      </View>
    </Modal>
  );
}
