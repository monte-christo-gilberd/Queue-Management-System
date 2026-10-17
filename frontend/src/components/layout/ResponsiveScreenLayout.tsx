import { Image } from 'expo-image';
import { router } from 'expo-router';
import { Fragment, useContext, useState } from 'react';
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
import { OwnerContext } from '@/components/layout/owner-context';

export type TabKey = 'home' | 'antrian' | 'riwayat' | 'profile';

/** Item menu profil (panel profil HP dan desktop memakai item yang sama). */
export type OwnerMenuItem =
  | 'create-queue'
  | 'dashboard'
  | 'admin-contact'
  | 'queues'
  | 'history'
  | 'logout'
  | 'profile'
  | 'antrian'
  | 'riwayat-saya'
  | 'join';

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
  /** Status mode gelap untuk toggle di collapsible. Belum ada implementasi: toggle hanya tampilan. */
  darkMode?: boolean;
  /** Dipanggil saat toggle mode gelap ditekan. Sementara tidak dipakai (toggle tidak melakukan apa-apa). */
  onToggleDarkMode?: () => void;
  /** Hanya desktop: isi mengisi tinggi layar dan tidak ikut scroll halaman (scroll diatur di dalam isi). */
  fullHeight?: boolean;
  children: React.ReactNode;
};

/** Link navigasi utama — hanya tampil di panel desktop (di HP sudah ada bottom tab). */
/** Menu khusus owner di panel profil (HP dan desktop). Customer tidak melihatnya. */
const OWNER_MENU: { key: OwnerMenuItem; label: string }[] = [
  { key: 'admin-contact', label: 'Admin Contact' },
  { key: 'queues', label: 'Daftar Antrian' },
  { key: 'history', label: 'Riwayat Antrian' },
];

/** Menu panel profil — dipakai SAMA di HP dan desktop. */
const LOGOUT_ITEM: { key: OwnerMenuItem; label: string } = { key: 'logout', label: 'Keluar' };

/** Item yang sama dengan navbar bawah HP. Dipakai panel desktop agar tidak ada yang hilang saat tanpa navbar. */
const NAV_ITEMS: { key: OwnerMenuItem; label: string }[] = [
  { key: 'dashboard', label: 'Beranda' },
  { key: 'antrian', label: 'Antrian Saya' },
  { key: 'join', label: 'Masuk Antrian' },
  { key: 'riwayat-saya', label: 'Riwayat Saya' },
  { key: 'profile', label: 'Profil' },
];

/** Tab navbar aktif -> item yang disorot di panel desktop. */
const TAB_TO_ITEM: Record<TabKey, OwnerMenuItem> = {
  home: 'dashboard',
  antrian: 'antrian',
  riwayat: 'riwayat-saya',
  profile: 'profile',
};

// eslint-disable-next-line @typescript-eslint/no-require-imports
const LOGO_HEADER = require('../../../logo/logo-header.svg');

/** Navigasi bawah HP: Beranda (kiri), tombol QR (tengah, overlay), Antrian (kanan). Admin & Riwayat ada di collapsible. */
const TABS: { key: TabKey; label: string; icon: IconName; href: '/dashboard' | '/antrian' | '/riwayat' | '/profile' }[] = [
  { key: 'home', label: 'Beranda', icon: 'home', href: '/dashboard' },
  { key: 'antrian', label: 'Antrian', icon: 'list', href: '/antrian' },
  { key: 'riwayat', label: 'Riwayat', icon: 'history', href: '/riwayat' },
  { key: 'profile', label: 'Profil', icon: 'person', href: '/profile' },
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
  onBack,
  activeTab,
  activeItem,
  darkMode = false,
  onToggleDarkMode,
  fullHeight = false,
  children,
}: ResponsiveScreenLayoutProps) {
  // Sama seperti HP: tombol kembali redup bila tidak ada layar sebelumnya (hindari "GO_BACK was not handled").
  const canGoBack = router.canGoBack();
  const isOwner = useContext(OwnerContext);
  // Tampilan mode gelap (ikon). Tema belum diterapkan ke layar: lihat catatan di ZUSTAND/UI.
  const [dark, setDark] = useState(darkMode);
  const toggleDark = () => {
    setDark((v) => !v);
    onToggleDarkMode?.();
  };
  // State tampilan murni: dropdown terbuka atau tidak, dan tinggi header (untuk panel penuh ke bawah).
  const [menuOpen, setMenuOpen] = useState(false);
  const [joinOpen, setJoinOpen] = useState(false);
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
          <View className="flex-row items-center gap-3">
            {onBack ? (
              <Pressable
                onPress={onBack}
                disabled={!canGoBack}
                hitSlop={12}
                accessibilityRole="button"
                accessibilityLabel="Kembali"
                accessibilityState={{ disabled: !canGoBack }}
                className={cn(!canGoBack && 'opacity-30')}>
                <Icon name="back" size={32} color={AppColors.white} />
              </Pressable>
            ) : null}
            {/* Logo bisa diklik untuk kembali ke dashboard. */}
            <Pressable
              className="flex-row items-center"
              accessibilityRole="link"
              accessibilityLabel="Kembali ke dashboard"
              onPress={() => router.navigate('/dashboard')}>
              {/* expo-image tidak dibungkus NativeWind → ukuran lewat style (rasio logo 220:56). */}
              <Image source={LOGO_HEADER} style={styles.logo} contentFit="contain" accessibilityLabel="Antrian" />
            </Pressable>
          </View>
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
                darkMode={dark}
                onToggleDarkMode={toggleDark}
                onPress={() => {
                  setMenuOpen(false);
                  onMenuItemPress?.('profile');
                }}
              />
              {NAV_ITEMS.map((item) => (
                <PanelItem
                  key={item.key}
                  label={item.label}
                  active={item.key === (activeItem ?? (activeTab ? TAB_TO_ITEM[activeTab] : undefined))}
                  onPress={() => {
                    setMenuOpen(false);
                    if (item.key === 'join') setJoinOpen(true);
                    else onMenuItemPress?.(item.key);
                  }}
                />
              ))}
              {isOwner ? (
                <>
                  <View className="my-2 h-px bg-scrim" />
                  <CreateQueueItem
                    onPress={() => {
                      setMenuOpen(false);
                      onMenuItemPress?.('create-queue');
                    }}
                  />
                  {OWNER_MENU.map((item) => (
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
                </>
              ) : null}
              <View className="my-2 h-px bg-scrim" />
              {[LOGOUT_ITEM].map((item) => (
                <PanelItem
                  key={item.key}
                  label={item.label}
                  onPress={() => {
                    setMenuOpen(false);
                    onMenuItemPress?.(item.key);
                  }}
                />
              ))}
            </View>
          </>
        ) : null}
      </View>

      {fullHeight ? (
        <View className="flex-1 items-center">
          <View className="w-full max-w-desktop flex-1 p-8">{children}</View>
        </View>
      ) : (
        <ScrollView className="flex-1" contentContainerClassName="items-center">
          <View className="w-full max-w-desktop gap-5 p-8">{children}</View>
        </ScrollView>
      )}

      <JoinQueueModals visible={joinOpen} onClose={() => setJoinOpen(false)} />
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
  darkMode = false,
  onToggleDarkMode,
  children,
}: ResponsiveScreenLayoutProps) {
  const insets = useSafeAreaInsets();
  const isOwner = useContext(OwnerContext);
  // Tampilan mode gelap (ikon). Tema belum diterapkan ke layar: lihat catatan di ZUSTAND/UI.
  const [dark, setDark] = useState(darkMode);
  const toggleDark = () => {
    setDark((v) => !v);
    onToggleDarkMode?.();
  };
  // Jarak navbar mengambang dari bawah layar (di atas safe area).
  const floatingBottom = Math.max(insets.bottom, 8) + 12;
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
            <PersonIcon size={32} color={AppColors.white} />
          </Pressable>
        </View>
        <View className="h-1 bg-primarySoft" />

        <ScrollView
          className="flex-1"
          contentContainerClassName={cn('gap-4 p-5', activeTab && 'pb-32')}>
          {children}
        </ScrollView>

        {activeTab ? (
          <View
            className="absolute left-4 right-4 flex-row rounded-3xl bg-background px-2 pt-2 shadow-lg"
            style={{ bottom: floatingBottom, paddingBottom: 8 }}
            onLayout={(e) => setTabBarHeight(e.nativeEvent.layout.height)}>
            {TABS.map((tab, index) => {
              const active = tab.key === activeTab;
              const color = active ? AppColors.dark : AppColors.disabled;
              return (
                <Fragment key={tab.key}>
                  {/* Ruang kosong di tengah untuk tombol melayang + labelnya. */}
                  {index === 2 ? (
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
            style={{ bottom: floatingBottom + tabBarHeight - 30 }}
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
                darkMode={dark}
                onToggleDarkMode={toggleDark}
                onPress={() => {
                  setPanelOpen(false);
                  onMenuItemPress?.('profile');
                }}
              />
              {isOwner ? (
                <CreateQueueItem
                  onPress={() => {
                    setPanelOpen(false);
                    onMenuItemPress?.('create-queue');
                  }}
                />
              ) : null}
              {(isOwner ? OWNER_MENU : []).map((item) => (
                <PanelItem
                  key={item.key}
                  label={item.label}
                  onPress={() => {
                    setPanelOpen(false);
                    onMenuItemPress?.(item.key);
                  }}
                />
              ))}
              {[LOGOUT_ITEM].map((item) => (
                <PanelItem
                  key={item.key}
                  label={item.label}
                  onPress={() => {
                    setPanelOpen(false);
                    onMenuItemPress?.(item.key);
                  }}
                />
              ))}
            </View>
          </View>
        ) : null}
      </View>
    </ResponsiveContainer>
  );
}

/* ----------------------------------- Shared ----------------------------------- */

/** Foto profil + nama di atas panel (ketuk untuk profil), dengan tombol mode gelap bulat di sebelahnya. */
function PanelHeader({
  userName,
  onPress,
  darkMode,
  onToggleDarkMode,
}: {
  userName: string;
  onPress: () => void;
  darkMode: boolean;
  onToggleDarkMode: () => void;
}) {
  return (
    <View className="mb-4 flex-row items-center gap-3">
      <Pressable
        onPress={onPress}
        accessibilityRole="button"
        accessibilityLabel="Buka profil"
        className="flex-1 flex-row items-center gap-3">
        <View className="h-14 w-14 items-center justify-center rounded-full bg-dark">
          <PersonIcon size={22} color={AppColors.white} />
        </View>
        <Text className="flex-shrink text-[18px] font-extrabold text-foreground">{userName}</Text>
      </Pressable>
      {/* Mode terang: ikon matahari. Mode gelap: ikon bulan sabit. */}
      <Pressable
        onPress={onToggleDarkMode}
        accessibilityRole="switch"
        accessibilityState={{ checked: darkMode }}
        accessibilityLabel={darkMode ? 'Mode gelap aktif' : 'Mode terang aktif'}
        className="h-11 w-11 items-center justify-center rounded-full bg-dark">
        <Text className="text-[20px] text-white">{darkMode ? '☾' : '☀'}</Text>
      </Pressable>
    </View>
  );
}

/** "Buat Antrian Baru" di panel owner (selalu tampil untuk owner). */
function CreateQueueItem({ onPress }: { onPress: () => void }) {
  return <PanelItem label="Buat Antrian Baru" onPress={onPress} />;
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
