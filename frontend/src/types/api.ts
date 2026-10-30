/**
 * Bentuk data dari backend (lihat FRONTEND_GENERATION_GUIDE.md Bagian 3).
 * Komponen hanya menerima data ini lewat props — tidak ada pemanggilan API di sini.
 */

export type Queue = {
  id: number;
  owner_id: number;
  queue_name: string;
  queue_desc: string | null;
  status: 'active' | 'closed';
  created_at: string;
  closed_at: string | null;
};

export type Ticket = {
  id: number;
  queue_id: number;
  ticket_number: number;
  sort_key: number;
  user_id: number;
  status: 'waiting' | 'completed' | 'cancelled' | 'skipped';
  skip_count: number;
  last_notified_at: string | null;
  created_at: string;
  completed_at: string | null;
  cancelled_at: string | null;
  /** ID counter yang memanggil tiket ini (0 = belum dipanggil). */
  called_counter_id?: number | null;
  /** Nama counter yang memanggil tiket ini; null bila counter default disembunyikan. */
  called_counter_name?: string | null;
  /** Waktu tiket dipanggil (null bila belum dipanggil). */
  called_at?: string | null;
};

/**
 * Tiket + data tambahan yang ada di mockup tetapi BELUM ada di backend.
 * TODO: konfirmasi sumber data (nama customer & counter tidak ada di objek Ticket).
 */
export type TicketEntry = {
  ticket: Ticket;
  customerName?: string;
  counterName?: string;
};

/**
 * Queue + data tambahan untuk daftar "Semua Antrian".
 * TODO: konfirmasi sumber data (nomor saat ini & counter tidak ada di objek Queue).
 */
export type QueueListItem = {
  queue: Queue;
  currentTicketNumber?: number;
  counterName?: string;
};

/**
 * Riwayat satu queue.
 * TODO: backend belum mendukung ini (belum ada endpoint history).
 */
export type HistoryItem = {
  queue: Queue;
};

/** Respons login dan register: `{ data: { token } }`. */
export type AuthResponse = {
  token: string;
};

/** Queue untuk pemegang tiket (tanpa owner_id). Dipakai oleh GET /queues/:id/status. */
export type PublicQueue = Omit<Queue, 'owner_id'>;

/** Respons GET /api/v1/queues/:id/status (owner atau pemegang tiket). */
export type QueueStatus = {
  queue: PublicQueue;
  my_ticket: Ticket | null;
  /** ticket_number terakhir yang selesai dilayani; null bila belum ada. */
  current_number: number | null;
  /** Nama counter yang memanggil nomor saat ini; null bila counter default disembunyikan / belum ada. */
  current_counter_name: string | null;
  /** Jumlah tiket waiting di depan tiket saya; null bila saya tidak sedang menunggu. */
  waiting_ahead: number | null;
  /** ticket_number sebelum yang sedang dilayani; null bila belum ada. */
  previous_number: number | null;
  /** ticket_number waiting pertama; null bila tidak ada yang menunggu. */
  next_number: number | null;
};

/** Respons QR owner: `{ action, queue_id, type, join_url, image_url, expires_at }`. */
export type QueueQR = {
  action: 'queue_qr';
  queue_id: number;
  type: 'static' | 'dynamic';
  join_url: string;
  image_url: string;
  /** Token dinamis kedaluwarsa; null untuk token statis. */
  expires_at: number | null;
};

/** Error QR dari backend. */
export type QueueQRErrorCode =
  | 'QR_INVALID'
  | 'QR_EXPIRED'
  | 'QR_TYPE_INVALID'
  | 'QR_UNAVAILABLE'
  | 'QR_REPLACE_NOT_ALLOWED';
