import { useErrorToast } from '@/components/ui/Toast';
import { router } from "expo-router";
import { useState } from "react";
import { Pressable, ScrollView, Text, View } from "react-native";

import { useIsTriple, useIsWide } from "@/hooks/use-is-wide";
import type { Queue } from "@/types/api";
import { cn } from "@/utils/cn";
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from "@/components/layout/ResponsiveScreenLayout";
import { useThemeColor } from "@/hooks/use-theme-mode";
import { AppButton } from "@/components/ui/AppButton";
import { Icon } from "@/components/ui/Icon";
import { EmptyState } from "@/components/ui/EmptyState";
import { FormField } from "@/components/ui/FormField";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { notePreview } from "@/utils/notes";
import { ModalCard } from "@/components/ui/ModalCard";
import { MySpotCard } from "@/components/MySpotCard";
import { AppendNoteRow, TimelineList, type TimelineItem } from "@/components/QueueTimeline";
import { InsertTicketModal } from "@/components/ui/InsertTicketModal";

export type QueueDetailProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  queue: Queue;
  queueName: string;
  queueDescription: string;
  onChangeQueueName: (value: string) => void;
  onChangeQueueDescription: (value: string) => void;
  /** Awalan nomor tiket (1-3 huruf/angka). Kolom tampil bila `onChangeTicketPrefix` diberikan. */
  ticketPrefix?: string;
  onChangeTicketPrefix?: (value: string) => void;
  /** Simpan nama dan deskripsi (PUT). Tombol hanya tampil bila diberikan. */
  onSave?: () => void;
  saving?: boolean;
  errorMessage?: string;

  /** Timeline semua tiket antrian (urut sort_key). */
  timeline: TimelineItem[];
  /** Nomor yang sedang dilayani, sudah diformat; null bila belum ada. */
  currentNumber: string | null;
  onSkip?: () => void;
  skipLoading?: boolean;
  onNext?: () => void;
  /** Panggil ulang nomor yang sedang dilayani (push ke pemiliknya). */
  onRecall?: () => void;
  recallDisabled?: boolean;
  recallLoading?: boolean;
  /** Tampilkan Panggil ulang di tempat Skip. */
  showRecall?: boolean;
  skipDisabled?: boolean;
  nextLoading?: boolean;
  /** Buka popup catatan untuk nomor yang sedang dilayani (popup dirender route). */
  onOpenNote?: () => void;
  /** Nomor yang boleh dihapus owner (waiting atau sedang dilayani), tampil di daftar tiket. */
  removableTickets?: { id: number; number: string; current: boolean; hasNote?: boolean }[];
  /** Nomor yang punya catatan owner (semua status), tampil di bagian Catatan pada daftar tiket. */
  noteItems?: { id: number; number: string; note: string }[];
  /** Buka popup catatan untuk nomor tertentu dari daftar tiket (isi lama dimuat dulu oleh route). */
  onOpenTicketNote?: (ticketId: number) => void;
  /**
   * Popup catatan sedang terbuka. Daftar tiket disembunyikan selama itu dan muncul lagi saat popup ditutup:
   * urutan lapisan dua modal bertumpuk di web ditentukan kapan portal-nya dibuat, jadi tidak bisa diandalkan.
   */
  noteOpen?: boolean;
  /** Hapus nomor customer dari antrian (4.10). Dipanggil setelah owner mengonfirmasi. */
  onRemoveTicket?: (ticketId: number) => void;
  removeLoading?: boolean;
  /** Sisip customer ke posisi tertentu (US-10). Mengembalikan true bila berhasil. */
  onInsert?: (identifier: string, position: number) => Promise<boolean>;
  insertLoading?: boolean;
  insertError?: string;
  /** Posisi awal sisip: panjang antrian menunggu + 1. */
  defaultInsertPosition?: number;

  /** Slot kartu QR (dirender route dari `useQueueQR`). */
  qrCard?: React.ReactNode;

  /** Slot kartu layar publik (dirender route dari `useDisplayConfig`). */
  displayCard?: React.ReactNode;

  /** Slot kartu pengaturan Skip (dirender route dari `useSkipSettings`). */
  skipSettingsCard?: React.ReactNode;
  /** Tab counter di atas timeline (hanya owner, dari CounterBar). */
  counterBar?: React.ReactNode;

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
  ticketPrefix = '',
  onChangeTicketPrefix,
  onSave,
  saving = false,
  errorMessage,
  timeline,
  currentNumber,
  onSkip,
  skipLoading = false,
  onNext,
  onRecall,
  recallDisabled = false,
  recallLoading = false,
  showRecall = false,
  skipDisabled = false,
  nextLoading = false,
  onOpenNote,
  removableTickets = [],
  noteItems = [],
  onOpenTicketNote,
  noteOpen = false,
  onRemoveTicket,
  removeLoading = false,
  onInsert,
  insertLoading = false,
  insertError,
  defaultInsertPosition = 1,
  qrCard,
  displayCard,
  skipSettingsCard,
  counterBar,
  onCloseQueue,
  closing = false,
}: QueueDetailProps) {
  useErrorToast(errorMessage);
  const isWide = useIsWide();
  const isTriple = useIsTriple();
  const isClosed = queue.status === "closed";
  const avatarColor = useThemeColor("avatar");

  // State tampilan murni: popup daftar tiket dan popup konfirmasi tutup.
  const [ticketsOpen, setTicketsOpen] = useState(false);
  const [insertOpen, setInsertOpen] = useState(false);
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<{ id: number; number: string } | null>(null);

  // Potongan kartu dipakai ulang oleh tata letak satu, dua, dan tiga kolom.
  const skipNextRow = !isClosed ? (
    <View className="flex-row gap-3">
      {showRecall && onRecall ? (
        <AppButton
          label="PANGGIL ULANG"
          variant="outline"
          className="flex-1"
          onPress={onRecall}
          loading={recallLoading}
          disabled={recallDisabled || !currentNumber}
        />
      ) : (
        <AppButton
          label="SKIP"
          variant="outline"
          className="flex-1"
          onPress={onSkip}
          loading={skipLoading}
          disabled={skipDisabled}
        />
      )}
      <AppButton label="NEXT" variant="dark" className="flex-1" onPress={onNext} loading={nextLoading} />
    </View>
  ) : null;

  // Tampilan customer: tanpa counter, sebelumnya, dan berikutnya. Ketuk untuk membuka daftar tiket.
  const spotCard = (
    <MySpotCard
      queueName={queueName}
      currentNumber={currentNumber}
      previousNumber={null}
      nextNumber={null}
      showNeighbors={false}
      onPress={() => setTicketsOpen(true)}
    />
  );

  const noteRow = (
    <View className="rounded-[20px] bg-dark p-3 dark:border dark:border-line">
      <AppendNoteRow onNote={onOpenNote} />
    </View>
  );

  const formCard = (
    <View className="gap-3 rounded-card border border-line p-3.5">
      <FormField label="Nama Antrian (bisa diedit disini)" value={queueName} onChangeText={onChangeQueueName} />
      <FormField label="Deskripsi Antrian" value={queueDescription} onChangeText={onChangeQueueDescription} />
      {onChangeTicketPrefix ? (
        <FormField
          label="Awalan Nomor"
          placeholder="mis. A, VIP"
          value={ticketPrefix}
          onChangeText={(v) => onChangeTicketPrefix(v.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 3))}
          autoCapitalize="characters"
        />
      ) : null}
      {onSave ? <AppButton label="SIMPAN" variant="dark" loading={saving} onPress={onSave} /> : null}
    </View>
  );

  const dangerCard = isClosed ? (
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
  ) : null;

  return (
    <ResponsiveScreenLayout
      title="Detail Antrian"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="queues"
      onBack={() => router.back()}
      fullHeight={isWide}
    >
      <View className={isWide ? "min-h-0 flex-1 flex-row gap-6" : "gap-4"}>
        {isTriple ? (
          <>
            {/* Tiga kolom: operasional, pengaturan layar dan Skip, lalu form dan tutup antrian. Tiap kolom bisa
                di-scroll sendiri, jadi layar yang lebih pendek tetap bisa menjangkau semua kartu. */}
            <ScrollColumn className="flex-1">
              {counterBar}
              {qrCard}
              {skipNextRow}
              {spotCard}
              {noteRow}
            </ScrollColumn>
            <ScrollColumn className="flex-1" panel>
              {displayCard}
              {skipSettingsCard}
            </ScrollColumn>
            <ScrollColumn className="flex-1" panel>
              {formCard}
              {dangerCard}
            </ScrollColumn>
          </>
        ) : (
          <>
            {/* Kolom kiri: QR, layar publik, Skip/Next, tampilan customer, dan catatan. */}
            {isWide ? (
              <ScrollColumn className="w-[38%]">
                {counterBar}
                {qrCard}
                {displayCard}
                {skipNextRow}
                {spotCard}
                {noteRow}
              </ScrollColumn>
            ) : (
              <View className="gap-4">
                {counterBar}
                {qrCard}
                {displayCard}
                {skipNextRow}
                {spotCard}
                {noteRow}
              </View>
            )}

            {/* Kolom kanan: pengaturan dan kontrol antrian. Di desktop satu panel yang di-scroll sendiri. */}
            <View className={isWide ? "min-h-0 flex-1" : "gap-4"}>
              <ScrollPanel enabled={isWide}>
                {formCard}
                {skipSettingsCard}
                {dangerCard}
              </ScrollPanel>
            </View>
          </>
        )}
      </View>

      {/* Daftar semua tiket yang terdaftar di antrian ini (dibuka dari kartu customer). */}
      <ModalCard visible={ticketsOpen && !noteOpen} onRequestClose={() => setTicketsOpen(false)}>
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
        {noteItems.length > 0 ? (
          <View className="gap-2">
            <Text className="text-[13px] font-semibold text-muted">Catatan</Text>
            {noteItems.map((n) => (
              <Pressable
                key={n.id}
                accessibilityRole="button"
                accessibilityLabel={`Edit catatan ${n.number}`}
                onPress={() => onOpenTicketNote?.(n.id)}
                className="gap-0.5 rounded-card border border-line p-3"
              >
                <View className="flex-row items-center justify-between">
                  <Text className="text-[15px] font-extrabold text-foreground">{n.number}</Text>
                  <Text className="text-[12px] text-muted">Edit</Text>
                </View>
                <Text className="text-[13px] text-foreground">{notePreview(n.note)}</Text>
              </Pressable>
            ))}
          </View>
        ) : null}
        {removableTickets.length > 0 ? (
          <View className="gap-2">
            <Text className="text-[13px] font-semibold text-muted">Hapus nomor dari antrian</Text>
            {removableTickets.map((t) => (
              <View key={t.id} className="flex-row items-center justify-between rounded-card border border-line p-3">
                <View>
                  <Text className="text-[15px] font-extrabold text-foreground">{t.number}</Text>
                  <Text className="text-[12px] text-muted">{t.current ? 'Sedang dilayani' : 'Menunggu'}</Text>
                </View>
                <View className="flex-row gap-2">
                  {onOpenTicketNote ? (
                    <AppButton
                      label={t.hasNote ? "Edit catatan" : "Tambah catatan"}
                      variant="outline"
                      className="h-10"
                      onPress={() => onOpenTicketNote(t.id)}
                    />
                  ) : null}
                  <AppButton
                    label="Hapus"
                    variant="outline"
                    className="h-10"
                    onPress={() => setRemoveTarget({ id: t.id, number: t.number })}
                  />
                </View>
              </View>
            ))}
          </View>
        ) : null}
        {onInsert ? (
          <AppButton
            label="Sisip customer"
            variant="dark"
            disabled={isClosed}
            onPress={() => setInsertOpen(true)}
          />
        ) : null}
        <AppButton label="Tutup" variant="outline" onPress={() => setTicketsOpen(false)} />
      </ModalCard>

      {onInsert ? (
        <InsertTicketModal
          visible={insertOpen}
          defaultPosition={defaultInsertPosition}
          saving={insertLoading}
          errorMessage={insertError}
          onClose={() => setInsertOpen(false)}
          onSubmit={onInsert}
        />
      ) : null}

      <ConfirmModal
        visible={removeTarget !== null}
        title={`Hapus ${removeTarget?.number ?? ''} dari antrian?`}
        message="Customer dikeluarkan dari antrian dan tidak dikembalikan. Nomor berikutnya naik bila nomor ini sedang dilayani."
        confirmLabel="Ya, Hapus"
        loading={removeLoading}
        onConfirm={() => {
          if (removeTarget) onRemoveTicket?.(removeTarget.id);
          setRemoveTarget(null);
        }}
        onCancel={() => setRemoveTarget(null)}
      />

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

    </ResponsiveScreenLayout>
  );
}

/**
 * Kolom kanan: di desktop satu panel bergaris yang di-scroll sendiri; di HP hanya tumpukan biasa.
 * Isi panel diposisikan absolut agar tingginya mengikuti kolom kiri (tidak menambah tinggi baris),
 * sehingga garis bawah kedua kolom selalu sejajar.
 */
/**
 * Satu kolom tata letak desktop yang bisa di-scroll sendiri. Tingginya mengikuti ruang yang tersedia (bukan isinya),
 * jadi isi yang lebih tinggi dari layar di-scroll alih-alih terpotong. `panel` memberi bingkai seperti panel kanan.
 */
function ScrollColumn({
  className,
  panel = false,
  children,
}: {
  className?: string;
  panel?: boolean;
  children: React.ReactNode;
}) {
  return (
    <View className={cn("relative min-h-0 overflow-hidden", panel && "rounded-card border border-line", className)}>
      <ScrollView
        className="absolute bottom-0 left-0 right-0 top-0"
        contentContainerClassName={cn("gap-4", panel && "p-4")}
        showsVerticalScrollIndicator={false}
      >
        {children}
      </ScrollView>
    </View>
  );
}

function ScrollPanel({ enabled, children }: { enabled: boolean; children: React.ReactNode }) {
  if (!enabled) return <View className="flex-1 gap-4">{children}</View>;
  return (
    <View className="relative flex-1 overflow-hidden rounded-card border border-line">
      <ScrollView className="absolute bottom-0 left-0 right-0 top-0" contentContainerClassName="gap-4 p-4">
        {children}
      </ScrollView>
    </View>
  );
}
