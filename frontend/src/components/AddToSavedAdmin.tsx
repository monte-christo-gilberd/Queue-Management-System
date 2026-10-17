import { router } from 'expo-router';
import { Text, View } from 'react-native';

import { useThemeColor } from '@/hooks/use-theme-mode';
import { useIsWide } from '@/hooks/use-is-wide';
import { cn } from '@/utils/cn';
import type { AdminContact } from '@/types/api';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';
import { AppButton } from '@/components/ui/AppButton';
import { Avatar } from '@/components/ui/Avatar';
import { EmptyState } from '@/components/ui/EmptyState';
import { PersonIcon } from '@/components/ui/Icon';
import { SearchBar } from '@/components/ui/SearchBar';

export type AddToSavedAdminProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  search: string;
  onChangeSearch: (value: string) => void;
  /** Hasil pencarian user terdaftar. Kosong + search terisi = tampilkan state "tidak ditemukan". */
  results: AdminContact[];
  /** TODO: backend belum mendukung ini (simpan admin ke kontak) */
  onAdd: (admin: AdminContact) => void;
};

export default function AddToSavedAdmin({
  shell,
  onMenuItemPress,
  search,
  onChangeSearch,
  results,
  onAdd,
}: AddToSavedAdminProps) {
  const avatarColor = useThemeColor('avatar');
  const isWide = useIsWide();
  const notFound = results.length === 0 && search.length > 0;

  return (
    <ResponsiveScreenLayout
      title="Add to Saved Admin"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="admin-contact"
      onBack={() => router.back()}>
      <SearchBar value={search} onChangeText={onChangeSearch} />

      {notFound ? (
        <EmptyState
          icon={<PersonIcon size={44} color={avatarColor} />}
          title="No Registered User with that Username / Email"
          description="Periksa kembali username atau email yang Anda masukkan"
        />
      ) : (
        <>
          {results.length > 0 ? <Text className="text-[15px] font-extrabold text-foreground">User Terdaftar</Text> : null}
          <View className={isWide ? 'flex-row flex-wrap gap-4' : 'gap-4'}>
            {results.map((user, index) => (
              <View
                key={user.id}
                className={cn(
                  'flex-row items-center gap-3 rounded-card border border-line p-3',
                  isWide && 'w-[48.5%]',
                )}>
                <Avatar active={index === 0} />
                <View className="flex-1 gap-0.5">
                  <Text className="text-[15px] font-extrabold text-foreground">{user.name}</Text>
                  <Text className="text-[12px] font-normal text-muted">{user.email}</Text>
                </View>
                <AppButton
                  label="Add"
                  variant="primary"
                  className="h-9 w-[76px] px-0"
                  onPress={() => onAdd(user)}
                />
              </View>
            ))}
          </View>
        </>
      )}
    </ResponsiveScreenLayout>
  );
}
