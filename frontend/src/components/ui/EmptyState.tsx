import { Text, View } from 'react-native';

type EmptyStateProps = {
  icon: React.ReactNode;
  title: string;
  description: string;
};

export function EmptyState({ icon, title, description }: EmptyStateProps) {
  return (
    <View className="items-center gap-2 rounded-card bg-surface px-6 py-14">
      {icon}
      <Text className="text-center text-[16px] font-extrabold text-foreground">{title}</Text>
      <Text className="text-center text-[12px] text-muted">{description}</Text>
    </View>
  );
}
