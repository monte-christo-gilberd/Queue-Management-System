import { router } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { useThemeColor } from '@/hooks/use-theme-mode';
import { useIsWide } from '@/hooks/use-is-wide';
import type { HistoryItem } from '@/types/api';
import { cn } from '@/utils/cn';
import { formatDateTime } from '@/utils/format';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';
import { EmptyState } from '@/components/ui/EmptyState';
import { Icon } from '@/components/ui/Icon';

export type HistoryProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  /** TODO: backend belum mendukung ini (belum ada endpoint history) */
  items: HistoryItem[];
};

export default function History({ shell, onMenuItemPress, items }: HistoryProps) {
  const avatarColor = useThemeColor('avatar');
  const foreground = useThemeColor('foreground');
  const isWide = useIsWide();
  const empty = items.length === 0;

  return (
    <ResponsiveScreenLayout
      title="History"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="history"
      onBack={() => router.back()}>


      {empty ? (
        <EmptyState
          icon={<Icon name="history" size={44} color={avatarColor} />}
          title="Tidak Ada History"
          description="Riwayat antrian akan muncul di sini setelah Anda selesai menggunakan antrian"
        />
      ) : (
        <View className={isWide ? 'flex-row flex-wrap gap-4' : 'gap-4'}>
          {items.map(({ queue, adminNames }) => (
            <View
              key={queue.id}
              className={cn(
                'flex-row items-center rounded-card border border-line p-3.5',
                isWide && 'w-[31.5%]',
              )}>
              <View className="flex-1 gap-[3px]">
                <Text className="text-[15px] font-extrabold text-foreground">{queue.queue_name}</Text>
                <Text className="text-[12px] font-normal text-muted">{formatDateTime(queue.created_at)}</Text>
                <Text className="text-[12px] font-semibold text-foreground">{adminNames.join(', ')}</Text>
              </View>
              <Pressable
                className="h-10 w-10 items-center justify-center rounded-[10px] bg-surface"
                onPress={() =>
                  router.push({ pathname: '/history/[queueId]', params: { queueId: queue.id } })
                }>
                <Icon name="search" size={20} color={foreground} />
              </Pressable>
            </View>
          ))}
        </View>
      )}
    </ResponsiveScreenLayout>
  );
}
