/**
 * Toast: sistem notifikasi galat sementara (ToastProvider, useErrorToast, ToastViewport).
 *
 * Dipakai oleh : app/_layout.tsx (ToastProvider), components/ui/ModalCard.tsx (viewport di dalam modal),
 *                dan banyak layar lewat useErrorToast
 * Bergantung   : hooks/use-is-wide (ToastViewport)
 * Alur         : ToastProvider menyimpan daftar toast -> useErrorToast/useToast memanggil show ->
 *                ToastViewport menampilkan toast di halaman atau di dalam modal paling atas
 * Catatan      : Toast hilang sendiri setelah TOAST_DURATION_MS. Pesan yang sama tidak ditumpuk.
 */
import { createContext, useCallback, useContext, useEffect, useId, useMemo, useRef, useState } from 'react';
import { Platform, Pressable, Text, View } from 'react-native';

import { useIsWide } from '@/hooks/use-is-wide';

/** Lama toast tampil sebelum hilang sendiri. */
export const TOAST_DURATION_MS = 5000;

/** Satu toast yang sedang tampil. id unik per toast; message sudah di-trim. */
type ToastItem = { id: number; message: string };

type ToastContextValue = {
  /** Toast yang sedang tampil, urut dari yang paling lama. */
  toasts: ToastItem[];
  /** Tampilkan pesan galat. Pesan yang sama dengan toast yang masih tampil tidak ditumpuk, hanya diperpanjang. */
  show: (message: string) => void;
  /** Tutup satu toast secara manual (hapus timer-nya juga). */
  dismiss: (id: number) => void;
  /**
   * Modal yang terbuka, urut dari yang pertama dibuka. Toast ditampilkan di dalam modal paling atas (bukan di
   * halaman) supaya tidak tertutup latar gelapnya, dan hanya satu viewport yang tampil walau modal bertumpuk.
   */
  modalStack: string[];
  /** Daftarkan modal terbuka ke modalStack. Mengembalikan fungsi untuk melepasnya lagi. */
  registerModal: (id: string) => () => void;
};

const noop = () => {};
const ToastContext = createContext<ToastContextValue>({
  toasts: [],
  show: noop,
  dismiss: noop,
  modalStack: [],
  registerModal: () => noop,
});

/** Penyedia toast untuk seluruh aplikasi (dipasang sekali di layout root). */
export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [modalStack, setModalStack] = useState<string[]>([]);
  // Cermin state agar show/dismiss tidak bergantung pada closure yang basi.
  const toastsRef = useRef<ToastItem[]>([]);
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>());
  const nextId = useRef(1);

  // Simpan daftar toast ke cermin dan state sekaligus, supaya keduanya selalu sama.
  const commit = useCallback((next: ToastItem[]) => {
    toastsRef.current = next;
    setToasts(next);
  }, []);

  const dismiss = useCallback(
    (id: number) => {
      const timer = timers.current.get(id);
      if (timer) clearTimeout(timer);
      timers.current.delete(id);
      commit(toastsRef.current.filter((t) => t.id !== id));
    },
    [commit],
  );

  // Mulai (atau mulai ulang) timer hilang otomatis untuk satu toast.
  const arm = useCallback(
    (id: number) => {
      const existing = timers.current.get(id);
      if (existing) clearTimeout(existing);
      timers.current.set(
        id,
        setTimeout(() => dismiss(id), TOAST_DURATION_MS),
      );
    },
    [dismiss],
  );

  const show = useCallback(
    (message: string) => {
      const text = message.trim();
      if (!text) return;
      // Pesan yang sama sudah tampil: hanya timer-nya diperpanjang, tidak ada toast baru.
      const same = toastsRef.current.find((t) => t.message === text);
      if (same) {
        arm(same.id);
        return;
      }
      const id = nextId.current++;
      commit([...toastsRef.current, { id, message: text }]);
      arm(id);
    },
    [arm, commit],
  );

  const registerModal = useCallback((id: string) => {
    setModalStack((stack) => [...stack, id]);
    return () => setModalStack((stack) => stack.filter((x) => x !== id));
  }, []);

  // Saat provider dilepas (unmount), semua timer dibersihkan supaya tidak ada callback yang jalan.
  useEffect(() => {
    const active = timers.current;
    return () => {
      active.forEach((timer) => clearTimeout(timer));
      active.clear();
    };
  }, []);

  const value = useMemo(
    () => ({ toasts, show, dismiss, modalStack, registerModal }),
    [toasts, show, dismiss, modalStack, registerModal],
  );
  return <ToastContext.Provider value={value}>{children}</ToastContext.Provider>;
}

/** Akses fungsi show dan dismiss toast dari komponen mana pun di bawah ToastProvider. */
export function useToast() {
  const { show, dismiss } = useContext(ToastContext);
  return { show, dismiss };
}

/**
 * Tampilkan `message` sebagai toast galat setiap kali ia berubah menjadi teks baru.
 * Pesan kosong tidak menampilkan apa pun. Menggantikan teks merah inline.
 */
export function useErrorToast(message?: string | null) {
  const { show } = useToast();
  useEffect(() => {
    if (message) show(message);
  }, [message, show]);
}

/** Dipakai ModalCard: menandai modal terbuka agar toast tampil di dalamnya. Mengembalikan id modal ini. */
export function useToastModalOpen(open: boolean): string {
  const { registerModal } = useContext(ToastContext);
  const id = useId();
  useEffect(() => {
    if (!open) return;
    // Cleanup saat modal ditutup atau unmount: id dilepas dari modalStack.
    return registerModal(id);
  }, [open, registerModal, id]);
  return id;
}

/**
 * Tumpukan toast di bawah layar. Toast terbaru muncul paling bawah dan yang lama bergeser ke atas,
 * jadi pesan tidak saling menutupi. `modalId` = viewport milik modal itu; hanya satu viewport yang tampil:
 * milik modal paling atas bila ada modal terbuka, selain itu milik halaman.
 */
export function ToastViewport({ modalId }: { modalId?: string }) {
  const { toasts, dismiss, modalStack } = useContext(ToastContext);
  const isWide = useIsWide();
  const owner = modalStack.length > 0 ? modalStack[modalStack.length - 1] : undefined;
  // Viewport ini tidak tampil bila tidak ada toast atau bila modal ini bukan pemilik viewport yang aktif.
  if (toasts.length === 0 || modalId !== owner) return null;

  return (
    <View
      pointerEvents="box-none"
      style={{
        // `fixed` di web supaya toast tetap di bawah jendela walau halaman di-scroll.
        position: (Platform.OS === 'web' ? 'fixed' : 'absolute') as 'absolute',
        left: 0,
        right: 0,
        // Di layar sempit, sisakan ruang untuk navbar bawah.
        bottom: isWide ? 24 : 88,
        alignItems: 'center',
        paddingHorizontal: 16,
        gap: 8,
        zIndex: 9999,
        elevation: 9999,
      }}
    >
      {toasts.map((toast) => (
        <View
          key={toast.id}
          accessibilityRole="alert"
          className="w-full max-w-[440px] flex-row items-center gap-3 rounded-card bg-live px-4 py-3"
        >
          <Text className="flex-1 text-[13px] font-semibold text-white">{toast.message}</Text>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Tutup pesan"
            onPress={() => dismiss(toast.id)}
            hitSlop={8}
          >
            <Text className="text-[18px] font-bold leading-[18px] text-white">×</Text>
          </Pressable>
        </View>
      ))}
    </View>
  );
}
