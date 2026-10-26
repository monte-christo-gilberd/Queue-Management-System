import { Modal, Pressable, View } from 'react-native';

type ModalCardProps = {
  visible: boolean;
  onRequestClose: () => void;
  children: React.ReactNode;
};

/** Modal putih di tengah layar dengan latar gelap, seperti "Append Note". */
export function ModalCard({ visible, onRequestClose, children }: ModalCardProps) {
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onRequestClose}>
      <Pressable className="flex-1 justify-center bg-modalBackdrop px-[30px]" onPress={onRequestClose}>
        <Pressable className="w-full max-w-[480px] self-center rounded-3xl bg-background p-5" onPress={() => {}}>
          <View className="gap-3.5 rounded-card">{children}</View>
        </Pressable>
      </Pressable>
    </Modal>
  );
}
