/**
 * ModalCard: kerangka popup (modal) bersama dengan latar gelap dan toast viewport di dalamnya.
 *
 * Dipakai oleh : components/CounterBar.tsx, components/MyHistoryList.tsx, components/QueueQRCard.tsx,
 *                components/CustomerQueueScreen.tsx, components/CreateQueueModal.tsx,
 *                components/ui/AppendNoteModal.tsx, components/ui/JoinQueueModals.tsx,
 *                components/ui/ConfirmModal.tsx, components/ui/MyTicketModal.tsx,
 *                components/ui/InsertTicketModal.tsx, components/Profile.tsx (foto profil diperbesar)
 * Bergantung   : components/ui/Toast (useToastModalOpen, ToastViewport), utils/cn
 * Catatan      : Tap latar gelap memanggil onRequestClose. Isi kartu hanya menelan tap, tidak menutup modal.
 *                Padding kartu 20 px; `compact` mengecilkannya jadi 8 px untuk isi yang memenuhi kartu (foto).
 *                Angka padding ini ditiru oleh utils/avatar-preview.ts; ubah keduanya bersama.
 */
import { Modal, Pressable, ScrollView, View } from 'react-native';

import { ToastViewport, useToastModalOpen } from '@/components/ui/Toast';
import { cn } from '@/utils/cn';

type ModalCardProps = {
  /** true = modal tampil (dengan animasi fade); false = modal disembunyikan. */
  visible: boolean;
  /** Dipanggil saat latar gelap ditekan atau tombol back perangkat. Penutupan dikerjakan induk lewat visible. */
  onRequestClose: () => void;
  /** Isi kartu putih di tengah. */
  children: React.ReactNode;
  /** true = padding kartu 8 px (bingkai tipis di sekeliling isi); false = 20 px. Default false. */
  compact?: boolean;
  /**
   * true = kartu dibatasi 90% tinggi layar dan isinya bisa di-scroll (untuk isi yang panjang, mis. daftar tiket).
   * false = kartu tumbuh sesuai isi. Default false.
   */
  scrollable?: boolean;
};

/** Modal putih di tengah layar dengan latar gelap, seperti "Append Note". */
export function ModalCard({ visible, onRequestClose, children, compact = false, scrollable = false }: ModalCardProps) {
  // Selama modal terbuka, toast galat tampil lewat viewport di dalam modal (di atas latar gelapnya).
  const toastId = useToastModalOpen(visible);
  // Kartu diberi onPress kosong agar tap di dalam kartu tidak sampai ke latar dan menutup modal.
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onRequestClose}>
      <View className="flex-1">
        <Pressable className="flex-1 justify-center bg-modalBackdrop px-[30px]" onPress={onRequestClose}>
          <Pressable
            className={cn('w-full max-w-[480px] self-center rounded-3xl bg-background', compact ? 'p-2' : 'p-5')}
            style={scrollable ? { maxHeight: '90%' } : undefined}
            onPress={() => {}}>
            {scrollable ? (
              <ScrollView style={{ flexShrink: 1 }} showsVerticalScrollIndicator={false}>
                <View className="gap-3.5 rounded-card">{children}</View>
              </ScrollView>
            ) : (
              <View className="gap-3.5 rounded-card">{children}</View>
            )}
          </Pressable>
        </Pressable>
        <ToastViewport modalId={toastId} />
      </View>
    </Modal>
  );
}
