import { Image } from 'expo-image';
import { router } from 'expo-router';
import { Fragment, useState } from 'react';
import {
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  useWindowDimensions,
  View,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { AppColors } from '@/constants/app-theme';
import { cn } from '@/utils/cn';
import { useHydrated } from '@/hooks/use-hydrated';
import { useIsWide } from '@/hooks/use-is-wide';
import ResponsiveContainer from '@/components/ResponsiveContainer';
import { Icon, PersonIcon, ScanIcon, type IconName } from '@/components/ui/Icon';
import { JoinQueueModals } from '@/components/ui/JoinQueueModals';
import { PageWatermark } from '@/components/ui/PageWatermark';

export type TabKey = 'home' | 'queues' | 'admins' | 'history';

/** Item menu profil (panel profil HP dan desktop memakai item yang sama). */
export type OwnerMenuItem =
  | 'dashboard'
  | 'admin-contact'
  | 'queues'
  | 'history'
  | 'logout'
  | 'profile'
  | 'settings';

/** Data tampilan untuk header/panel profil — diisi developer. */
export type ShellProps = {
  userName?: string;
};

export type ResponsiveScreenLayoutProps = {
  title: string;
  userName?: string;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  /** Hanya desktop: link navigasi utama yang di-highlight di panel. */
  activeItem?: OwnerMenuItem;
  /** Hanya dipakai di HP: tombol back muncul kalau diberikan. */
  onBack?: () => void;
  /** Hanya dipakai di HP: tab bawah yang aktif. */
  activeTab?: TabKey;
  /** Hanya HP: ikon profil di header berwarna biru (halaman Edit Profil). */
  profileHighlighted?: boolean;
  /** Status mode gelap untuk toggle di collapsible. Belum ada implementasi: toggle hanya tampilan. */
  darkMode?: boolean;
  /** Dipanggil saat toggle mode gelap ditekan. Sementara tidak dipakai (toggle tidak melakukan apa-apa). */
  onToggleDarkMode?: () => void;
  children: React.ReactNode;
};

/** Link navigasi utama — hanya tampil di panel desktop (di HP sudah ada bottom tab). */
const WIDE_NAV: { key: OwnerMenuItem; label: string }[] = [
  { key: 'dashboard', label: 'Dashboard' },
  { key: 'admin-contact', label: 'Admin Contact' },
  { key: 'queues', label: 'Daftar Antrian' },
];

/** Menu panel profil — dipakai SAMA di HP dan desktop. */
const PROFILE_MENU: { key: OwnerMenuItem; label: string }[] = [
  { key: 'settings', label: 'Pengaturan' },
  { key: 'history', label: 'Riwayat' },
  { key: 'logout', label: 'Keluar' },
];

// eslint-disable-next-line @typescript-eslint/no-require-imports
const LOGO_HEADER = require('../../../logo/logo-header.svg');

/** Navigasi bawah HP: Beranda (kiri), tombol QR (tengah, overlay), Antrian (kanan). Admin & Riwayat ada di collapsible. */
const TABS: { key: TabKey; label: string; icon: IconName; href: '/dashboard' | '/queues' }[] = [
  { key: 'home', label: 'Beranda', icon: 'home', href: '/dashboard' },
  { key: 'queues', label: 'Antrian', icon: 'list', href: '/queues' },
];

/** Item tambahan di collapsible HP (yang dulu ada di navigasi bawah). Riwayat sudah ada di PROFILE_MENU. */
const MOBILE_NAV: { key: OwnerMenuItem; label: string }[] = [
  { key: 'admin-contact', label: 'Admin Contact' },
];

/**
 * Kerangka semua halaman owner. Responsif secara internal:
 * - lebar >= 1024: header dengan panel profil (sama dengan HP), konten maks 1200 ditengahkan, tanpa back/tab bar.
 * - lebih sempit: header HP (back + judul + profil), bottom tab bar, panel profil.
 */
export default function ResponsiveScreenLayout(props: ResponsiveScreenLayoutProps) {
  const isWide = useIsWide();
  const hydrated = useHydrated();

  // Render statis (web) memakai lebar bawaan HP; tunggu klien siap agar layout HP/bottom tab
  // tidak sempat tampil sekilas di layar desktop.
  if (!hydrated) return <View className="flex-1 bg-background" />;

  return isWide ? <WideLayout {...props} /> : <MobileLayout {...props} />;
}

/* ------------------------------- Wide (desktop) ------------------------------- */

function WideLayout({
  title,
  userName = 'Nama User',
  onMenuItemPress,
  activeItem,
  darkMode = false,
  onToggleDarkMode,
  children,
}: ResponsiveScreenLayoutProps) {
  // State tampilan murni: dropdown terbuka atau tidak, dan tinggi header (untuk panel penuh ke bawah).
  const [menuOpen, setMenuOpen] = useState(false);
  const [headerHeight, setHeaderHeight] = useState(0);
  const { height: windowHeight } = useWindowDimensions();

  // Panel menutupi layar penuh dari paling atas. Web: `position: fixed` menempel ke viewport;
  // native: absolute relatif ke header, digeser ke atas setinggi header. Nilainya dinamis/tidak ada
  // padanan Tailwind (`fixed`, tinggi layar terukur) sehingga tetap lewat style.
  const overlayPosition =
    Platform.OS === 'web'
      ? ({ position: 'fixed', top: 0, bottom: 0 } as unknown as { position: 'absolute' })
      : { top: -headerHeight, height: windowHeight };

  return (
    <View className="flex-1 bg-background">
      <PageWatermark />
      <View className="z-10" onLayout={(e) => setHeaderHeight(e.nativeEvent.layout.height)}>
        <View className="min-h-[64px] flex-row items-center justify-between bg-dark px-6 py-3">
          {/* Logo bisa diklik untuk kembali ke dashboard. */}
          <Pressable
            className="flex-row items-center"
            accessibilityRole="link"
            accessibilityLabel="Kembali ke dashboard"
            onPress={() => router.navigate('/dashboard')}>
            {/* expo-image tidak dibungkus NativeWind → ukuran lewat style (rasio logo 220:56). */}
            <Image source={LOGO_HEADER} style={styles.logo} contentFit="contain" accessibilityLabel="Antrian" />
          </Pressable>
          {/* Judul ditengahkan terhadap seluruh header (tidak terpengaruh lebar logo / nama user). */}
          <View
            className="absolute bottom-0 left-0 right-0 top-0 items-center justify-center"
            pointerEvents="none">
            <Text className="text-[17px] font-bold text-white">{title}</Text>
          </View>
          <View className="flex-row items-center gap-3.5">
            <Text className="text-[17px] font-bold text-white">{userName}</Text>
            <Pressable hitSlop={12} onPress={() => setMenuOpen((open) => !open)}>
              <PersonIcon size={32} color={AppColors.white} />
            </Pressable>
          </View>
        </View>
        <View className="h-1 bg-primarySoft" />

        {menuOpen ? (
          <>
            <Pressable
              className="absolute left-0 right-0 bg-scrim"
              style={overlayPosition}
              onPress={() => setMenuOpen(false)}
            />
            <View className="absolute right-0 w-80 bg-panel px-4 pt-3" style={overlayPosition}>
              <PanelHeader
                userName={userName}
                onPress={() => {
                  setMenuOpen(false);
                  onMenuItemPress?.('profile');
                }}
              />
              {WIDE_NAV.map((item) => (
                <PanelItem
                  key={item.key}
                  label={item.label}
                  active={item.key === activeItem}
                  onPress={() => {
                    setMenuOpen(false);
                    onMenuItemPress?.(item.key);
                  }}
                />
              ))}
              <View className="my-2 h-px bg-scrim" />
              {PROFILE_MENU.map((item) => (
                <PanelItem
                  key={item.key}
                  label={item.label}
                  onPress={() => {
                    setMenuOpen(false);
                    onMenuItemPress?.(item.key);
                  }}
                />
              ))}
              <View className="my-2 h-px bg-scrim" />
              <DarkModeToggle value={darkMode} onPress={onToggleDarkMode} />
            </View>
          </>
        ) : null}
      </View>

      <ScrollView className="flex-1" contentContainerClassName="items-center">
        <View className="w-full max-w-desktop gap-5 p-8">{children}</View>
      </ScrollView>
    </View>
  );
}

/* --------------------------------- Mobile (HP) --------------------------------- */

function MobileLayout({
  title,
  userName = 'Owner',
  onMenuItemPress,
  onBack,
  activeTab,
  profileHighlighted = false,
  darkMode = false,
  onToggleDarkMode,
  children,
}: ResponsiveScreenLayoutProps) {
  const insets = useSafeAreaInsets();
  // State tampilan murni: panel profil sedang terbuka atau tidak, dan tinggi tab bar (untuk menaruh tombol tengah).
  const [panelOpen, setPanelOpen] = useState(false);
  const [tabBarHeight, setTabBarHeight] = useState(0);
  const [joinOpen, setJoinOpen] = useState(false);
  // Tombol back nonaktif (redup) bila tidak ada layar sebelumnya — mencegah error "GO_BACK was not handled".
  const canGoBack = router.canGoBack();

  return (
    // Di tablet (di bawah 1024) konten HP tetap dibatasi 720 dan ditengahkan.
    <ResponsiveContainer maxWidth={720}>
      <View className="flex-1 bg-background">
        <PageWatermark />
        {/* Padding atas mengikuti safe-area (nilai runtime) → lewat style. */}
        <View
          className="flex-row items-center bg-dark px-4 pb-3"
          style={{ paddingTop: insets.top + 8 }}>
          <View className="w-11 items-center justify-center">
            {onBack ? (
              <Pressable
                onPress={onBack}
                disabled={!canGoBack}
                hitSlop={12}
                accessibilityRole="button"
                accessibilityLabel="Kembali"
                accessibilityState={{ disabled: !canGoBack }}
                className={cn(!canGoBack && 'opacity-30')}>
                <Icon name="back" size={40} color={AppColors.white} />
              </Pressable>
            ) : null}
          </View>
          <Text className="flex-1 text-center text-[17px] font-bold text-white">{title}</Text>
          <Pressable
            onPress={() => setPanelOpen(true)}
            hitSlop={12}
            className="w-11 items-center justify-center">
            <PersonIcon
              size={32}
              color={profileHighlighted ? AppColors.primarySoft : AppColors.white}
            />
          </Pressable>
        </View>
        <View className="h-1 bg-primarySoft" />

        <ScrollView
          className="flex-1"
          contentContainerClassName={cn('gap-4 p-5', activeTab && 'pb-12')}>
          {children}
        </ScrollView>

        {activeTab ? (
          <View
            className="flex-row border-t border-divider bg-background pt-2"
            style={{ paddingBottom: Math.max(insets.bottom, 8) }}
            onLayout={(e) => setTabBarHeight(e.nativeEvent.layout.height)}>
            {TABS.map((tab, index) => {
              const active = tab.key === activeTab;
              const color = active ? AppColors.dark : AppColors.disabled;
              return (
                <Fragment key={tab.key}>
                  {/* Ruang kosong di tengah untuk tombol melayang + labelnya. */}
                  {index === 1 ? (
                    <View className="w-20 items-center justify-end">
                      <Text className="text-[11px] font-semibold text-foreground">Masuk</Text>
                    </View>
                  ) : null}
                  <Pressable
                    className="flex-1 items-center gap-0.5"
                    onPress={() => router.navigate(tab.href)}>
                    <Icon name={tab.icon} size={26} color={color} />
                    <Text
                      className={cn(
                        'text-[11px]',
                        active ? 'font-extrabold text-foreground' : 'font-semibold text-disabled',
                      )}>
                      {tab.label}
                    </Text>
                  </Pressable>
                </Fragment>
              );
            })}
          </View>
        ) : null}

        {/* Tombol "Masuk ke Antrian" gaya QR mbanking: bulat, di tengah, separuh di atas tab bar.
            Dirender sebagai overlay (bukan anak tab bar) agar tetap bisa disentuh di Android. */}
        {activeTab ? (
          <View
            className="absolute left-0 right-0 items-center"
            style={{ bottom: tabBarHeight - 30 }}
            pointerEvents="box-none">
            <Pressable
              onPress={() => setJoinOpen(true)}
              accessibilityRole="button"
              accessibilityLabel="Masuk ke antrian"
              className="h-[60px] w-[60px] items-center justify-center rounded-full border-4 border-background bg-primary shadow-lg active:opacity-90">
              <ScanIcon size={24} color={AppColors.white} />
            </Pressable>
          </View>
        ) : null}

        {/* Popup scan QR / link manual; submit sementara hanya menutup popup. */}
        <JoinQueueModals visible={joinOpen} onClose={() => setJoinOpen(false)} />

        {panelOpen ? (
          <View style={StyleSheet.absoluteFill}>
            <Pressable
              className="absolute bottom-0 left-0 right-0 top-0 bg-scrim"
              onPress={() => setPanelOpen(false)}
            />
            <View
              className="absolute bottom-0 right-0 top-0 w-3/4 bg-panel px-4"
              style={{ paddingTop: insets.top + 12 }}>
              <PanelHeader
                userName={userName}
                onPress={() => {
                  setPanelOpen(false);
                  onMenuItemPress?.('profile');
                }}
              />
              {MOBILE_NAV.map((item) => (
                <PanelItem
                  key={item.key}
                  label={item.label}
                  onPress={() => {
                    setPanelOpen(false);
                    onMenuItemPress?.(item.key);
                  }}
                />
              ))}
              {PROFILE_MENU.map((item) => (
                <PanelItem
                  key={item.key}
                  label={item.label}
                  onPress={() => {
                    setPanelOpen(false);
                    onMenuItemPress?.(item.key);
                  }}
                />
              ))}
              <View className="my-2 h-px bg-scrim" />
              <DarkModeToggle value={darkMode} onPress={onToggleDarkMode} />
            </View>
          </View>
        ) : null}
      </View>
    </ResponsiveContainer>
  );
}

/* ----------------------------------- Shared ----------------------------------- */

/** Toggle "Mode Gelap" — placeholder: tampilan saja, belum mengubah tema (implementasi nanti). */
function DarkModeToggle({ value, onPress }: { value: boolean; onPress?: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="switch"
      accessibilityState={{ checked: value }}
      accessibilityLabel="Mode gelap"
      className="flex-row items-center justify-between px-1 py-3.5">
      <Text className="text-[15px] font-bold text-foreground">Mode Gelap</Text>
      <View
        className={cn('h-6 w-11 justify-center rounded-full px-0.5', value ? 'bg-primary' : 'bg-line')}>
        <View className={cn('h-5 w-5 rounded-full bg-white', value ? 'self-end' : 'self-start')} />
      </View>
    </Pressable>
  );
}

/** Foto profil + nama di atas panel; bisa diklik untuk membuka halaman Profil. */
function PanelHeader({ userName, onPress }: { userName: string; onPress: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel="Buka profil"
      className="mb-4 flex-row items-center gap-3">
      <View className="h-14 w-14 items-center justify-center rounded-full bg-dark">
        <PersonIcon size={22} color={AppColors.white} />
      </View>
      <Text className="text-[18px] font-extrabold text-foreground">{userName}</Text>
    </Pressable>
  );
}

function PanelItem({
  label,
  onPress,
  active = false,
}: {
  label: string;
  onPress: () => void;
  active?: boolean;
}) {
  return (
    <Pressable
      onPress={onPress}
      className={cn('rounded-input px-1 py-3.5', active && 'bg-panelActive')}>
      <Text className="text-[15px] font-bold text-foreground">{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  // expo-image: tidak dibungkus NativeWind.
  logo: { width: 141, height: 36 },
});
