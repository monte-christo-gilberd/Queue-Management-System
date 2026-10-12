import { router } from "expo-router";
import { useState } from "react";
import { Text, View } from "react-native";

import { useIsWide } from "@/hooks/use-is-wide";
import type { Queue, TicketEntry } from "@/types/api";
import { cn } from "@/utils/cn";
import { formatTicketNumber } from "@/utils/format";
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from "@/components/layout/ResponsiveScreenLayout";
import { AppButton } from "@/components/ui/AppButton";
import { AppendNoteModal } from "@/components/ui/AppendNoteModal";
import { JoinQueueModals } from "@/components/ui/JoinQueueModals";
import { CurrentNumberCard } from "@/components/ui/CurrentNumberCard";

export type DashboardProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  /** null/undefined = belum ada antrian aktif (tampilan kosong). */
  activeQueue?: Queue | null;
  previous?: TicketEntry | null;
  current?: TicketEntry | null;
  next?: TicketEntry | null;
  /** Tiket-tiket setelah `next`. */
  upcoming?: TicketEntry[];
  /** Teks notifikasi apa adanya, mis. "Nomor A-042 dipanggil di Counter 2". */
  notificationMessage?: string;
  nextLoading?: boolean;
  errorMessage?: string;
  /** TODO: backend belum mendukung ini (skip ticket) */
  onSkip?: () => void;
  onNext?: () => void;
  onCreateQueue?: () => void;
  onJoinQueue?: () => void;
  /** TODO: backend belum mendukung ini (catatan antrian) */
  onSubmitNote?: (note: string) => void;
};

function EntryRow({
  label,
  entry,
  highlighted = false,
}: {
  label: string;
  entry?: TicketEntry | null;
  highlighted?: boolean;
}) {
  return (
    <View
      className={cn(
        "flex-row justify-between p-3.5",
        highlighted && "bg-primary",
      )}
    >
      <Text
        className={cn(
          "text-[12px] font-normal",
          highlighted ? "text-white" : "text-muted",
        )}
      >
        {label}
      </Text>
      {/* TODO: konfirmasi sumber data — nama customer tidak ada di objek Ticket */}
      <Text
        className={cn(
          "text-[12px] font-extrabold",
          highlighted ? "text-white" : "text-foreground",
        )}
      >
        {entry
          ? `${formatTicketNumber(entry.ticket.ticket_number)}${entry.customerName ? ` · ${entry.customerName}` : ""}`
          : "-"}
      </Text>
    </View>
  );
}

export default function Dashboard({
  shell,
  onMenuItemPress,
  activeQueue,
  previous,
  current,
  next,
  upcoming = [],
  notificationMessage,
  nextLoading = false,
  errorMessage,
  onSkip,
  onNext,
  onCreateQueue,
  onJoinQueue,
  onSubmitNote,
}: DashboardProps) {
  const isWide = useIsWide();
  // State tampilan murni: modal catatan sedang terbuka atau tidak.
  const [noteOpen, setNoteOpen] = useState(false);
  // State tampilan murni: popup scan QR / link manual (dibuka tombol "Masuk Ke Antrian" di PC).
  const [joinOpen, setJoinOpen] = useState(false);

  return (
    <ResponsiveScreenLayout
      title="Dashboard"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="dashboard"
      activeTab="home"
      onBack={() => router.back()}
    >
      {activeQueue ? (
        <View className={isWide ? "flex-row items-start gap-6" : "gap-4"}>
          <View className={cn("gap-4", isWide && "flex-1")}>
            <CurrentNumberCard
              numberLabel={
                current ? formatTicketNumber(current.ticket.ticket_number) : "-"
              }
              counterName={current?.counterName}
              showLiveDot
              onPressNote={() => setNoteOpen(true)}
            />
            <View className="flex-row gap-3">
              <AppButton
                label="SKIP"
                variant="outline"
                className="flex-1"
                onPress={onSkip}
              />
              <AppButton
                label="NEXT"
                variant="dark"
                className="flex-1"
                onPress={onNext}
                loading={nextLoading}
              />
            </View>

            {errorMessage ? (
              <Text className="text-[12px] font-normal text-live">
                {errorMessage}
              </Text>
            ) : null}

            {notificationMessage ? (
              <View className="flex-row items-center gap-2.5 rounded-card border border-line px-3.5 py-3">
                <View className="h-2.5 w-2.5 rounded-full bg-live" />
                <View>
                  <Text className="text-[11px] text-muted">
                    Notifikasi Nomor saat Ini
                  </Text>
                  <Text className="text-[14px] font-bold text-foreground">
                    {notificationMessage}
                  </Text>
                </View>
              </View>
            ) : null}
          </View>

          <View className={isWide ? "flex-1" : undefined}>
            <EntryRow label="Nama dan Nomor Sebelumnya" entry={previous} />
            <EntryRow
              label="Nama dan Nomor Saat Ini"
              entry={current}
              highlighted
            />
            <EntryRow label="Nama dan Nomor Selanjutnya" entry={next} />
            {upcoming.map((entry) => (
              <EntryRow
                key={entry.ticket.id}
                label="Nama dan Nomor Selanjutnya"
                entry={entry}
              />
            ))}
          </View>
        </View>
      ) : (
        <View className="items-center gap-2 rounded-card bg-surface p-5">
          <View className="mb-1 flex-row gap-2.5">
            <View className="h-[30px] w-[30px] rounded-full bg-live" />
            <View className="h-[30px] w-[30px] rounded-full bg-avatar" />
            <View className="h-[30px] w-[30px] rounded-full bg-avatar" />
          </View>
          <Text className="text-[15px] font-extrabold text-foreground">
            Belum ada antrian yang aktif
          </Text>
          <Text className="text-center text-[12px] font-normal text-muted">
            Buat antrian baru atau masuk ke antrian yang tersedia.
          </Text>
        </View>
      )}

      {/* TODO: tidak ada mockup layar "Buat Antrian Baru" — tujuan navigasi diisi developer */}
      <View className={isWide ? "flex-row gap-3" : "gap-4"}>
        <AppButton
          label="Buat Antrian Baru"
          variant="soft"
          className={isWide ? "flex-1" : undefined}
          onPress={onCreateQueue}
        />
        {/* HP: "Masuk ke Antrian" pindah jadi tombol tengah di navigasi bawah (lihat layout). */}
        {isWide ? (
          <AppButton
            label="Masuk Ke Antrian"
            variant="dark"
            className="flex-1"
            onPress={() => {
              setJoinOpen(true);
              onJoinQueue?.();
            }}
          />
        ) : null}
      </View>

      <JoinQueueModals visible={joinOpen} onClose={() => setJoinOpen(false)} />

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
