import { Modal, Pressable, View } from 'react-native';

import { ToastViewport, useToastModalOpen } from '@/components/ui/Toast';

type ModalCardProps = {
  visible: boolean;
  onRequestClose: () => void;
  children: React.ReactNode;
};

/** Modal putih di tengah layar dengan latar gelap, seperti "Append Note". */
export function ModalCard({ visible, onRequestClose, children }: ModalCardProps) {
  // Selama modal terbuka, toast galat tampil lewat viewport di dalam modal (di atas latar gelapnya).
  const toastId = useToastModalOpen(visible);
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
