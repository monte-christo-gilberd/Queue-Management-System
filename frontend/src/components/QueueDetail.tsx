import { router } from "expo-router";
import { useState } from "react";
import { Pressable, Text, View } from "react-native";

import { useIsWide } from "@/hooks/use-is-wide";
import type { Queue, TicketEntry } from "@/types/api";
import { cn } from "@/utils/cn";
import { formatTicketNumber } from "@/utils/format";
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from "@/components/layout/ResponsiveScreenLayout";
import { AppButton } from "@/components/ui/AppButton";
import { CurrentNumberCard } from "@/components/ui/CurrentNumberCard";
import { FormField } from "@/components/ui/FormField";
import { ModalCard } from "@/components/ui/ModalCard";

export type QueueDetailProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  queue: Queue;
  current?: TicketEntry | null;
  queueName: string;
  queueDescription: string;
  onChangeQueueName: (value: string) => void;
  onChangeQueueDescription: (value: string) => void;
  /** Nama admin yang punya otoritas atas queue ini. TODO: backend belum mendukung ini (queue_admins) */
  adminNames?: string[];
  /** TODO: backend belum mendukung ini (skip ticket) */
  onSkip?: () => void;
  onNext?: () => void;
  nextLoading?: boolean;
  /** TODO: backend belum mendukung ini (tambah admin by nama + email) */
  onSubmitAddAdmin?: (name: string, email: string) => void;
  errorMessage?: string;
  /** Simpan nama dan deskripsi (PUT). Tombol hanya tampil bila diberikan. */
  onSave?: () => void;
  saving?: boolean;
  /** Tutup antrian (POST /close). Dipanggil setelah pengguna mengonfirmasi di popup. */
  onCloseQueue?: () => void;
  closing?: boolean;
  /** Tautan join untuk dibagikan (`<origin>/join/<id>`). Null = tidak tersedia (mis. native tanpa EXPO_PUBLIC_WEB_URL). */
  joinLink?: string | null;
  onCopyLink?: () => void;
  linkCopied?: boolean;
};

export default function QueueDetail({
  shell,
  onMenuItemPress,
  queue,
  current,
  queueName,
  queueDescription,
  onChangeQueueName,
  onChangeQueueDescription,
  adminNames = [],
  onSkip,
  onNext,
  nextLoading = false,
  onSubmitAddAdmin,
  errorMessage,
  onSave,
  saving = false,
  onCloseQueue,
  closing = false,
  joinLink,
  onCopyLink,
  linkCopied = false,
}: QueueDetailProps) {
  const isWide = useIsWide();
  // State tampilan murni: modal "Add Admin" + isi form sebelum di-submit.
  const [addAdminOpen, setAddAdminOpen] = useState(false);
  const [adminName, setAdminName] = useState("");
  const [adminEmail, setAdminEmail] = useState("");
  // State tampilan murni: popup konfirmasi tutup antrian.
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const isClosed = queue.status === "closed";

  const closeModal = () => {
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
    >
      <View className={isWide ? "flex-row items-start gap-6" : "gap-4"}>
        <View className={cn("gap-4", isWide && "flex-1")}>
          <Text className="text-[15px] font-extrabold text-foreground">
            Data Nomor Saat Ini
          </Text>

          <CurrentNumberCard
            compact
            numberLabel={
              current ? formatTicketNumber(current.ticket.ticket_number) : "-"
            }
            counterName={current?.counterName}
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
        </View>

        <View className={cn("gap-4", isWide && "flex-1")}>
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
              <AppButton
                label="SIMPAN"
                variant="dark"
                loading={saving}
                onPress={onSave}
              />
            ) : null}
            <View className="gap-1.5">
              <Text className="text-[12px] font-semibold text-muted">
                Otoritas Admin (dengan centang)
              </Text>
              <Pressable
                className="h-11 items-center justify-center rounded-input bg-surface"
                onPress={() => router.push("/admins/add-to-queue")}
              >
                <Text className="text-[14px] font-extrabold text-foreground">
                  {adminNames.join(", ") || "-"}
                </Text>
              </Pressable>
              <Pressable onPress={() => setAddAdminOpen(true)}>
                <Text className="text-[13px] font-bold text-primary">
                  + Tambah Admin
                </Text>
              </Pressable>
            </View>
          </View>

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
                  label={linkCopied ? 'TERSALIN' : 'SALIN TAUTAN'}
                  variant="outline"
                  onPress={onCopyLink}
                />
              ) : null}
            </View>
          ) : null}

          {isClosed ? (
            <Text className="text-[13px] font-bold text-muted">Antrian ini sudah ditutup.</Text>
          ) : onCloseQueue ? (
            <AppButton
              label="TUTUP ANTRIAN"
              variant="outline"
              onPress={() => setCloseConfirmOpen(true)}
            />
          ) : null}

          {/* TODO: backend belum mendukung ini (sistem pengulangan antrian & sistem QR) */}
          <View className="gap-3 rounded-card border border-line p-3.5">
            <View className="gap-1.5">
              <Text className="text-[12px] font-semibold text-muted">
                Sistem Pengulangan antrian
              </Text>
              <Text className="text-[13px] font-bold text-foreground">
                mundur 2 kali · ke belakang · langsung batal
              </Text>
            </View>
            <View className="gap-1.5">
              <Text className="text-[12px] font-semibold text-muted">
                Sistem QR
              </Text>
              <Text className="text-[13px] font-bold text-foreground">
                static · dynamic
              </Text>
            </View>
          </View>
        </View>
      </View>

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

      <ModalCard visible={addAdminOpen} onRequestClose={closeModal}>
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
            closeModal();
          }}
        />
        <AppButton label="Cancel" variant="outline" onPress={closeModal} />
      </ModalCard>
    </ResponsiveScreenLayout>
  );
}
