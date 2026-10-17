import { useState } from "react";
import { Text, View } from "react-native";

import type { Queue, TicketEntry } from "@/types/api";
import { formatTicketNumber } from "@/utils/format";
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from "@/components/layout/ResponsiveScreenLayout";
import { AppendNoteModal } from "@/components/ui/AppendNoteModal";
import { QueueTimeline } from "@/components/QueueTimeline";

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
  nextLoading?: boolean;
  errorMessage?: string;
  /** TODO: backend belum mendukung ini (skip ticket) */
  onSkip?: () => void;
  onNext?: () => void;
  /** Kartu di atas papan: kondisi antrian saya (customer). */
  topCard?: React.ReactNode;
  /** TODO: backend belum mendukung ini (catatan antrian) */
  onSubmitNote?: (note: string) => void;
  /** Buka detail antrian (mis. /queues/:id). */
  onOpenQueue?: (queueId: number) => void;
  /** Konten tambahan di bawah papan (mis. daftar tiket saya). */
  footer?: React.ReactNode;
  /** Owner: true (default). Customer: false, sehingga bagian kosong owner dan tombol buat antrian disembunyikan. */
  canCreateQueue?: boolean;
};

export default function Dashboard({
  shell,
  onMenuItemPress,
  activeQueue,
  previous,
  current,
  next,
  upcoming = [],
  nextLoading = false,
  errorMessage,
  onSkip,
  onNext,
  topCard,
  onSubmitNote,
  onOpenQueue,
  footer,
  canCreateQueue = true,
}: DashboardProps) {
  // State tampilan murni: modal catatan sedang terbuka atau tidak.
  const [noteOpen, setNoteOpen] = useState(false);

  return (
    <ResponsiveScreenLayout
      title="Dashboard"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="dashboard"
      activeTab="home"
    >
      {topCard}

      {activeQueue ? (
        <View className="w-full gap-4">
          {errorMessage ? (
            <Text className="text-[12px] font-normal text-live">{errorMessage}</Text>
          ) : null}
          <QueueTimeline
            title={activeQueue.queue_name}
            items={[
              ...(previous ? [{ number: formatTicketNumber(previous.ticket.ticket_number), status: "done" as const }] : []),
              ...(current ? [{ number: formatTicketNumber(current.ticket.ticket_number), status: "current" as const }] : []),
              ...(next ? [{ number: formatTicketNumber(next.ticket.ticket_number), status: "waiting" as const }] : []),
              ...upcoming.map((entry) => ({
                number: formatTicketNumber(entry.ticket.ticket_number),
                status: "waiting" as const,
              })),
            ]}
            onSkip={onSkip}
            onNext={onNext}
            onNote={() => setNoteOpen(true)}
            onOpen={onOpenQueue ? () => onOpenQueue(activeQueue.id) : undefined}
            nextLoading={nextLoading}
          />
        </View>
      ) : canCreateQueue ? (
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
      ) : null}


      {footer}


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
