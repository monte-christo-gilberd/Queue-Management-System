import { router } from "expo-router";
import { useState } from "react";
import { Pressable, ScrollView, Switch, Text, View } from "react-native";

import { useIsWide } from "@/hooks/use-is-wide";
import type { Queue, QueueAuthz } from "@/types/api";
import { cn } from "@/utils/cn";
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from "@/components/layout/ResponsiveScreenLayout";
import { AppColors } from "@/constants/app-theme";
import { useThemeColor } from "@/hooks/use-theme-mode";
import { AppButton } from "@/components/ui/AppButton";
import { Icon, PersonIcon } from "@/components/ui/Icon";
import { EmptyState } from "@/components/ui/EmptyState";
import { AppendNoteModal } from "@/components/ui/AppendNoteModal";
import { FormField } from "@/components/ui/FormField";
import { ModalCard } from "@/components/ui/ModalCard";
import { MySpotCard } from "@/components/MySpotCard";
import { AppendNoteRow, TimelineList, type TimelineItem } from "@/components/QueueTimeline";

type AuthzKey = keyof Omit<QueueAuthz, "queue_id">;

/** Izin admin yang tampil sebagai toggle, dengan label Indonesia. */
const AUTHZ_ITEMS: { key: AuthzKey; label: string }[] = [
  { key: "can_manage_queue", label: "Kelola antrian" },
  { key: "can_view_queue", label: "Lihat antrian" },
  { key: "can_continue_queue", label: "Lanjutkan antrian" },
  { key: "can_manage_tickets", label: "Kelola tiket" },
  { key: "can_manage_notes", label: "Kelola catatan" },
  { key: "can_notify_customer", label: "Notifikasi customer" },
  { key: "can_manage_counters", label: "Kelola counter" },
  { key: "can_manage_display", label: "Kelola display" },
  { key: "can_manage_qr", label: "Kelola QR" },
];

export type QueueDetailProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  queue: Queue;
  queueName: string;
  queueDescription: string;
  onChangeQueueName: (value: string) => void;
  onChangeQueueDescription: (value: string) => void;
  /** Simpan nama dan deskripsi (PUT). Tombol hanya tampil bila diberikan. */
  onSave?: () => void;
  saving?: boolean;
  errorMessage?: string;

  /** Timeline semua tiket antrian (urut sort_key). */
  timeline: TimelineItem[];
  /** Nomor yang sedang dilayani, sudah diformat; null bila belum ada. */
  currentNumber: string | null;
  /** TODO: backend belum mendukung ini (skip ticket) */
  onSkip?: () => void;
  onNext?: () => void;
  nextLoading?: boolean;
  /** TODO: backend belum mendukung ini (catatan antrian) */
  onSubmitNote?: (note: string) => void;

  /** Tautan join untuk dibagikan (`<origin>/join/<id>`). Null = tidak tersedia (mis. native tanpa EXPO_PUBLIC_WEB_URL). */
  joinLink?: string | null;
  onCopyLink?: () => void;
  linkCopied?: boolean;

  /** Nama admin yang punya otoritas atas queue ini. TODO: backend belum mendukung ini (queue_admins) */
  adminNames?: string[];
  /** TODO: backend belum mendukung ini (tambah admin by nama + email) */
  onSubmitAddAdmin?: (name: string, email: string) => void;

  /** Izin (authz) pada antrian ini. Undefined = kartu izin tidak tampil. */
  authz?: QueueAuthz;
  onToggleAuthz?: (key: AuthzKey, value: boolean) => void;
  onSaveAuthz?: () => void;
  savingAuthz?: boolean;

  /** Tutup antrian (POST /close). Dipanggil setelah pengguna mengonfirmasi di popup. */
  onCloseQueue?: () => void;
  closing?: boolean;
};

export default function QueueDetail({
  shell,
  onMenuItemPress,
  queue,
  queueName,
  queueDescription,
  onChangeQueueName,
  onChangeQueueDescription,
  onSave,
  saving = false,
  errorMessage,
  timeline,
  currentNumber,
  onSkip,
  onNext,
  nextLoading = false,
  onSubmitNote,
  joinLink,
  onCopyLink,
  linkCopied = false,
  adminNames = [],
  onSubmitAddAdmin,
  authz,
  onToggleAuthz,
  onSaveAuthz,
  savingAuthz = false,
  onCloseQueue,
  closing = false,
}: QueueDetailProps) {
  const isWide = useIsWide();
  const isClosed = queue.status === "closed";
  const avatarColor = useThemeColor("avatar");

  // State tampilan murni: popup catatan, popup konfirmasi tutup, dan popup tambah admin.
  const [noteOpen, setNoteOpen] = useState(false);
  const [ticketsOpen, setTicketsOpen] = useState(false);
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const [addAdminOpen, setAddAdminOpen] = useState(false);
  const [adminName, setAdminName] = useState("");
  const [adminEmail, setAdminEmail] = useState("");
  // TODO: backend belum mendukung pengaturan antrian; toggle ini hanya tampilan.
  const [loopEnabled, setLoopEnabled] = useState(false);
  const [dynamicQrEnabled, setDynamicQrEnabled] = useState(false);

  const closeAddAdmin = () => {
    setAddAdminOpen(false);
    setAdminName("");
    setAdminEmail("");
  };

  return (
    <ResponsiveScreenLayout
      title="Detail Antrian"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="queues"
      onBack={() => router.back()}
      fullHeight={isWide}
    >
      <View className={isWide ? "flex-1 flex-row gap-6" : "gap-4"}>
        {/* Kolom kiri: tautan, papan antrian, dan tampilan customer. Di desktop tidak di-scroll. */}
        <View className={cn("gap-4", isWide && "w-[38%]")}>
          {joinLink !== undefined && !isClosed ? (
            <View className="gap-2 rounded-card border border-line p-3.5">
              <Text className="text-[12px] font-semibold text-muted">Tautan join untuk customer</Text>
              {joinLink ? (
                <Text selectable className="text-[13px] font-bold text-foreground">
                  {joinLink}
                </Text>
              ) : (
                <Text className="text-[13px] text-muted">Tautan tidak tersedia di perangkat ini.</Text>
              )}
              {joinLink && onCopyLink ? (
                <AppButton
                  label={linkCopied ? "TERSALIN" : "SALIN TAUTAN"}
                  variant="outline"
                  onPress={onCopyLink}
                />
              ) : null}
            </View>
          ) : null}

          {!isClosed ? (
            <View className="flex-row gap-3">
              <AppButton label="SKIP" variant="outline" className="flex-1" onPress={onSkip} />
              <AppButton
                label="NEXT"
                variant="dark"
                className="flex-1"
                onPress={onNext}
                loading={nextLoading}
              />
            </View>
          ) : null}

          {errorMessage ? (
            <Text className="text-[12px] font-normal text-live">{errorMessage}</Text>
          ) : null}

          <View className="rounded-[20px] bg-dark p-3 dark:border dark:border-line">
            <AppendNoteRow onNote={() => setNoteOpen(true)} />
          </View>

          {/* Tampilan customer: tanpa counter, sebelumnya, dan berikutnya. Ketuk untuk membuka daftar tiket. */}
          <MySpotCard
            queueName={queueName}
            currentNumber={currentNumber}
            previousNumber={null}
            nextNumber={null}
            showNeighbors={false}
            onPress={() => setTicketsOpen(true)}
          />
        </View>

        {/* Kolom kanan: pengaturan dan kontrol antrian. Di desktop satu panel yang di-scroll sendiri. */}
        <View className={isWide ? "flex-1 gap-4" : "gap-4"}>
        <ScrollPanel enabled={isWide}>
          <View className="gap-3 rounded-card border border-line p-3.5">
            <FormField
              label="Nama Antrian (bisa diedit disini)"
              value={queueName}
              onChangeText={onChangeQueueName}
            />
            <FormField
              label="Deskripsi Antrian"
              value={queueDescription}
              onChangeText={onChangeQueueDescription}
            />
            {onSave ? (
              <AppButton label="SIMPAN" variant="dark" loading={saving} onPress={onSave} />
            ) : null}
          </View>


          {authz ? (
            <View className="gap-3 rounded-card border border-line p-3.5">
              <Text className="text-[12px] font-semibold text-muted">Otoritas admin</Text>
              {AUTHZ_ITEMS.map((item) => (
                <View key={item.key} className="flex-row items-center justify-between">
                  <Text className="text-[14px] text-foreground">{item.label}</Text>
                  <Switch
                    value={authz[item.key]}
                    onValueChange={(value) => onToggleAuthz?.(item.key, value)}
                    disabled={!onToggleAuthz}
                  />
                </View>
              ))}
              {onSaveAuthz ? (
                <AppButton label="SIMPAN IZIN" variant="dark" loading={savingAuthz} onPress={onSaveAuthz} />
              ) : null}
            </View>
          ) : null}

          {/* TODO: backend belum mendukung pengaturan antrian; toggle hanya tampilan. */}
          <View className="gap-3 rounded-card border border-line p-3.5">
            <Text className="text-[12px] font-semibold text-muted">Pengaturan antrian</Text>
            <View className="flex-row items-center justify-between">
              <View className="flex-1 gap-0.5 pr-3">
                <Text className="text-[14px] font-bold text-foreground">Sistem pengulangan antrian</Text>
                <Text className="text-[12px] text-muted">mundur 2 kali · ke belakang · langsung batal</Text>
              </View>
              <Switch value={loopEnabled} onValueChange={setLoopEnabled} />
            </View>
            <View className="flex-row items-center justify-between">
              <View className="flex-1 gap-0.5 pr-3">
                <Text className="text-[14px] font-bold text-foreground">Sistem QR</Text>
                <Text className="text-[12px] text-muted">static · dynamic</Text>
              </View>
              <Switch value={dynamicQrEnabled} onValueChange={setDynamicQrEnabled} />
            </View>
          </View>

          {isClosed ? (
            <Text className="text-[13px] font-bold text-muted">Antrian ini sudah ditutup.</Text>
          ) : onCloseQueue ? (
            <View className="gap-3 rounded-card border border-live bg-live/10 p-3.5">
              <Text className="text-[12px] font-semibold text-live">Zona berbahaya</Text>
              <Text className="text-[13px] text-foreground">
                Customer tidak bisa lagi bergabung, dan antrian tidak bisa dibuka kembali.
              </Text>
              <Pressable
                onPress={() => setCloseConfirmOpen(true)}
                className="h-12 items-center justify-center rounded-button bg-live active:opacity-80"
              >
                <Text className="text-[16px] font-bold text-white">TUTUP ANTRIAN</Text>
              </Pressable>
            </View>
          ) : null}
        </ScrollPanel>

        {/* Admin di bawah panel utama (di luar kartu), seperti deretan counter pada contoh. */}
        <View className="gap-2">
          <Text className="text-[12px] font-semibold text-muted">Admin</Text>
          <View className="gap-2 rounded-card border border-line bg-surface p-3">
            <ScrollView horizontal showsHorizontalScrollIndicator contentContainerClassName="gap-3">
              {adminNames.map((name) => (
                <View key={name} className="w-[120px] flex-row items-center gap-2 rounded-card bg-primary p-2">
                  <PersonIcon size={28} color={AppColors.white} />
                  <View className="flex-1 rounded-input bg-background px-2 py-2">
                    <Text numberOfLines={2} className="text-center text-[12px] font-bold text-foreground">
                      {name}
                    </Text>
                  </View>
                </View>
              ))}
              <Pressable
                onPress={() => setAddAdminOpen(true)}
                accessibilityRole="button"
                accessibilityLabel="Tambah admin"
                className="h-[72px] w-[72px] items-center justify-center rounded-full bg-primary active:opacity-80"
              >
                <Text className="text-[36px] font-bold text-white">+</Text>
              </Pressable>
            </ScrollView>
            {adminNames.length === 0 ? (
              <EmptyState
                icon={<PersonIcon size={44} color={avatarColor} />}
                title="Belum Ada Admin"
                description="Tambahkan admin untuk membantu mengelola antrian ini."
              />
            ) : null}
          </View>
        </View>
        </View>
      </View>

      {/* Daftar semua tiket yang terdaftar di antrian ini (dibuka dari kartu customer). */}
      <ModalCard visible={ticketsOpen} onRequestClose={() => setTicketsOpen(false)}>
        <Text className="text-[20px] font-extrabold text-foreground">Daftar tiket</Text>
        {timeline.length > 0 ? (
          <View className="rounded-[20px] bg-dark p-5">
            <TimelineList items={timeline} />
          </View>
        ) : (
          <EmptyState
            icon={<Icon name="list" size={44} color={avatarColor} />}
            title="Belum Ada Tiket"
            description="Tiket akan muncul di sini setelah customer bergabung ke antrian."
          />
        )}
        <AppButton label="Tutup" variant="outline" onPress={() => setTicketsOpen(false)} />
      </ModalCard>

      <ModalCard visible={closeConfirmOpen} onRequestClose={() => setCloseConfirmOpen(false)}>
        <Text className="text-[20px] font-extrabold text-foreground">Tutup antrian ini?</Text>
        <Text className="text-[13px] text-muted">
          Customer tidak bisa lagi bergabung, dan antrian tidak bisa dibuka kembali.
        </Text>
        <AppButton
          label="Ya, Tutup Antrian"
          variant="dark"
          loading={closing}
          onPress={() => {
            onCloseQueue?.();
            setCloseConfirmOpen(false);
          }}
        />
        <AppButton label="Batal" variant="outline" onPress={() => setCloseConfirmOpen(false)} />
      </ModalCard>

      <ModalCard visible={addAdminOpen} onRequestClose={closeAddAdmin}>
        <Text className="text-[24px] font-extrabold text-foreground">Add Admin</Text>
        <FormField
          label="Nama User"
          placeholder="Nama admin baru"
          value={adminName}
          onChangeText={setAdminName}
        />
        <FormField
          label="Email User"
          placeholder="email@mail.com"
          value={adminEmail}
          onChangeText={setAdminEmail}
          autoCapitalize="none"
          keyboardType="email-address"
        />
        <AppButton
          label="Tambah Admin"
          variant="dark"
          onPress={() => {
            onSubmitAddAdmin?.(adminName, adminEmail);
            closeAddAdmin();
          }}
        />
        <AppButton label="Cancel" variant="outline" onPress={closeAddAdmin} />
      </ModalCard>

      <AppendNoteModal
        visible={noteOpen}
        onCancel={() => setNoteOpen(false)}
        onSubmit={(note) => {
          onSubmitNote?.(note);
          setNoteOpen(false);
        }}
      />
    </ResponsiveScreenLayout>
  );
}

/** Kolom kanan: di desktop satu panel bergaris yang di-scroll sendiri; di HP hanya tumpukan biasa. */
function ScrollPanel({ enabled, children }: { enabled: boolean; children: React.ReactNode }) {
  if (!enabled) return <View className="flex-1 gap-4">{children}</View>;
  return (
    <View className="flex-1 overflow-hidden rounded-card border border-line">
      <ScrollView className="flex-1" contentContainerClassName="gap-4 p-4">
        {children}
      </ScrollView>
    </View>
  );
}
