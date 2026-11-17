/**
 * api: tipe data respons backend untuk antrian, tiket, riwayat, pengaturan skip, dan QR.
 *
 * Dipakai oleh : src/lib/*.ts (tipe kembalian fungsi API), src/hooks/*, src/components/*,
 *                src/utils/queue-view.ts, src/utils/history.ts.
 * Catatan      : Bentuk mengikuti DTO di backend/internal/dto/*.go. Komponen hanya menerima data ini
 *                lewat props; tidak ada pemanggilan API di sini.
 */

/** Queue: antrian milik satu owner, berisi nama, prefix nomor, dan status buka atau tutup. */
export type Queue = {
  /** ID antrian di database. */
  id: number;
  /** ID user owner pemilik antrian. */
  owner_id: number;
  /** Nama antrian yang tampil di daftar dan layar. */
  queue_name: string;
  /** Deskripsi antrian; null bila owner tidak mengisinya. */
  queue_desc: string | null;
  /** Prefix nomor tiket (1-3 huruf/angka besar), mis. "A". */
  ticket_prefix?: string;
  /** active = masih menerima tiket; closed = sudah ditutup owner. */
  status: 'active' | 'closed';
  /** Waktu antrian dibuat, format ISO 8601 (UTC). */
  created_at: string;
  /** Waktu antrian ditutup, format ISO 8601; null selama antrian masih aktif. */
  closed_at: string | null;
};

/** Ticket: satu nomor antrian milik satu customer, lengkap dengan status dan waktu tiap tahap. */
export type Ticket = {
  /** ID tiket di database. */
  id: number;
  /** ID antrian tempat tiket ini berada. */
  queue_id: number;
  /** Nomor tiket yang dilihat customer, tanpa prefix (prefix ada di Queue.ticket_prefix). */
  ticket_number: number;
  /** Urutan tiket di dalam antrian; nilai lebih kecil didahulukan saat memilih nomor berikutnya. */
  sort_key: number;
  /** ID customer pemilik tiket. */
  user_id: number;
  /** currently_served: nomor yang sedang dilayani. deleted: tiket yang dikeluarkan owner lewat remove. */
  status: 'waiting' | 'currently_served' | 'completed' | 'cancelled' | 'skipped' | 'deleted';
  /** Berapa kali nomor ini sudah dilewati (Skip). */
  skip_count: number;
  /** Waktu terakhir customer diberi tahu, format ISO 8601; null bila belum pernah. */
  last_notified_at: string | null;
  /** Waktu tiket dibuat (customer bergabung ke antrian), format ISO 8601. */
  created_at: string;
  /** Waktu nomor selesai dilayani, format ISO 8601; null bila belum selesai. */
  completed_at: string | null;
  /** Waktu tiket dibatalkan customer, format ISO 8601; null bila tidak dibatalkan. */
  cancelled_at: string | null;
  /** ID counter yang memanggil tiket ini; null bila belum pernah dipanggil. */
  called_counter_id?: number | null;
  /** Nama counter yang memanggil tiket ini; null bila counter default disembunyikan. */
  called_counter_name?: string | null;
  /** Catatan owner; hanya ada di daftar tiket owner (GET /queues/:id/tickets), tidak pernah di respons customer. */
  note?: string | null;
  /** Waktu catatan terakhir diubah, format ISO 8601; null bila belum ada catatan. */
  note_updated_at?: string | null;
  /** Waktu tiket dipanggil (null bila belum dipanggil). */
  called_at?: string | null;
};

/**
 * TicketEntry: tiket + data tambahan untuk tampilan layar owner.
 * TODO: customerName belum diisi pemanggil mana pun, karena objek Ticket dari backend tidak memuat nama customer
 * (nama customer hanya ada di respons riwayat, lihat HistoryTicket.customer_name). counterName diisi dari
 * called_counter_name oleh deriveQueueView.
 */
export type TicketEntry = {
  /** Tiket asli dari backend. */
  ticket: Ticket;
  /** Nama customer untuk ditampilkan; undefined karena sumbernya belum ada (lihat TODO di atas). */
  customerName?: string;
  /** Nama counter yang memanggil; undefined untuk counter default atau bila belum dipanggil. */
  counterName?: string;
};

/**
 * QueueListItem: antrian + data tambahan untuk daftar "Semua Antrian".
 * TODO: currentTicketNumber dan counterName belum diisi. app/(user)/queues/index.tsx hanya mengisi queue,
 * sedangkan QueueList.tsx sudah siap menampilkan keduanya bila diisi.
 */
export type QueueListItem = {
  /** Antrian yang ditampilkan. */
  queue: Queue;
  /** Nomor yang sedang dilayani (tanpa prefix); undefined bila belum ada atau belum diisi. */
  currentTicketNumber?: number;
  /** Nama counter yang melayani nomor tersebut; undefined untuk counter default. */
  counterName?: string;
};

/** Hasil akhir satu nomor di riwayat. `unserved` = masih menunggu saat antrian ditutup. */
export type HistoryOutcome = 'completed' | 'skipped' | 'cancelled' | 'deleted' | 'unserved';

/** Ringkasan antrian yang sudah ditutup (GET /api/v1/history/queues). Hitungan per hasil menjumlah ke `total`. */
export type HistoryQueue = {
  /** ID antrian di database. */
  id: number;
  /** Nama antrian. */
  queue_name: string;
  /** Prefix nomor tiket antrian ini, mis. "A" di "A-007". */
  ticket_prefix: string;
  /** Waktu antrian dibuat, format ISO 8601. */
  opened_at: string;
  /** Waktu antrian ditutup, format ISO 8601. */
  closed_at: string;
  /** Jumlah seluruh nomor di antrian ini. */
  total: number;
  /** Jumlah nomor yang selesai dilayani. */
  completed: number;
  /** Jumlah nomor yang berakhir sebagai dilewati. */
  skipped: number;
  /** Jumlah nomor yang dibatalkan customer. */
  cancelled: number;
  /** Jumlah nomor yang dikeluarkan owner. */
  deleted: number;
  /** Jumlah nomor yang masih menunggu saat antrian ditutup. */
  unserved: number;
};

/** Satu nomor di antrian yang sudah ditutup (GET /api/v1/history/queues/:id). Hanya dibaca owner. */
export type HistoryTicket = {
  /** ID tiket di database. */
  ticket_id: number;
  /** Nomor tiket tanpa prefix. */
  ticket_number: number;
  /** Hasil akhir nomor ini. */
  outcome: HistoryOutcome;
  /** Berapa kali nomor ini sudah dilewati. */
  skip_count: number;
  /** Waktu customer bergabung, format ISO 8601. */
  joined_at: string;
  /** Waktu nomor terakhir dipanggil, format ISO 8601; null bila belum pernah dipanggil. */
  called_at: string | null;
  /** Waktu nomor selesai dilayani, format ISO 8601; null bila tidak selesai. */
  completed_at: string | null;
  /** Waktu dibatalkan customer, format ISO 8601; null bila tidak dibatalkan. */
  cancelled_at: string | null;
  /** Nama customer pemilik tiket. */
  customer_name: string;
  /** Email customer pemilik tiket. */
  customer_email: string;
  /** Hanya terisi bila antrian punya lebih dari satu counter. */
  counter_name: string | null;
  /** Catatan owner; tidak pernah dilihat customer. */
  note: string | null;
};

/** Respons login dan register: `{ data: { token } }`. */
export type AuthResponse = {
  /** Token sesi; disimpan di authStore dan dikirim sebagai `Authorization: Bearer`. */
  token: string;
};

/** Queue untuk pemegang tiket (tanpa owner_id). Dipakai oleh GET /queues/:id/status. */
export type PublicQueue = Omit<Queue, 'owner_id'>;

/** Respons GET /api/v1/queues/:id/status (owner atau pemegang tiket). */
export type QueueStatus = {
  /** Data antrian, tanpa owner_id. */
  queue: PublicQueue;
  /** Tiket milik pemanggil di antrian ini; null bila pemanggil tidak punya tiket. */
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
  /** Semua counter yang sedang melayani, yang paling baru dipanggil dulu. */
  serving?: { counter_id: number; counter_name: string | null; ticket_number: number }[];
};

/** Apa yang terjadi pada nomor yang dilewati: remove = dikeluarkan dari antrian, back = kembali ke paling belakang, after_n = masuk di posisi N+1. */
export type SkipFallback = 'remove' | 'back' | 'after_n';
/** Pengaturan nomor yang dilewati (Skip). `n` hanya terisi untuk `after_n`. */
export type SkipSettings = {
  /** Lihat SkipFallback untuk arti tiap nilai. */
  fallback: SkipFallback;
  /** Posisi kembali untuk after_n (mis. 3 = masuk di urutan ke-4); null untuk fallback lain. */
  n: number | null;
  /** Berapa kali satu nomor boleh dikembalikan sebelum menjadi skipped permanen. */
  max_returns: number;
};

/** Respons QR owner. QR digambar frontend dari `join_url`. */
export type QueueQR = {
  /** Penanda jenis respons, selalu 'queue_qr'. */
  action: 'queue_qr';
  /** ID antrian pemilik QR ini. */
  queue_id: number;
  /** static = QR tetap; dynamic = token berubah dan kedaluwarsa. */
  type: 'static' | 'dynamic';
  /** URL yang dikodekan ke dalam QR untuk bergabung ke antrian. */
  join_url: string;
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

/** Satu tiket di riwayat customer (GET /api/v1/users/me/history). Tanpa catatan owner dan data user lain. */
export type MyHistoryTicket = {
  /** ID tiket di database. */
  ticket_id: number;
  /** ID antrian tempat tiket ini berada. */
  queue_id: number;
  /** Nama antrian. */
  queue_name: string;
  /** Prefix nomor tiket antrian ini, mis. "A". */
  ticket_prefix: string;
  /** Nomor tiket tanpa prefix. */
  ticket_number: number;
  /** Hasil akhir nomor ini, dalam bahasa internal (lihat customerOutcomeLabel untuk teks tampilan). */
  outcome: HistoryOutcome;
  /** Antrian sudah ditutup (tiket yang belum dilayani saat itu berhasil `unserved`). */
  queue_closed: boolean;
  /** Berapa kali nomor ini sudah dilewati. */
  skip_count: number;
  /** Waktu customer bergabung, format ISO 8601. */
  joined_at: string;
  /** Waktu nomor terakhir dipanggil, format ISO 8601; null bila belum pernah dipanggil. */
  called_at: string | null;
  /** Waktu nomor selesai dilayani, format ISO 8601; null bila tidak selesai. */
  completed_at: string | null;
  /** Waktu dibatalkan, format ISO 8601; null bila tidak dibatalkan. */
  cancelled_at: string | null;
  /** Hanya terisi bila antrian punya lebih dari satu counter. */
  counter_name: string | null;
};
