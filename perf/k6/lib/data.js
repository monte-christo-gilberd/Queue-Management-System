// data.js: memuat akun, token, dan antrian seed ke SharedArray (satu salinan di memori untuk semua VU).
import { SharedArray } from 'k6/data';
import { TOKENS_PATH } from './config.js';

// Satu kali baca dan parse berkas besar; SharedArray membagikannya ke semua VU tanpa menggandakan memori.
// Fungsi dijalankan sekali di init, dan masing-masing array mengurai JSON yang sama (open() di-cache oleh k6).
export const owners = new SharedArray('owners', () => JSON.parse(open(TOKENS_PATH)).owners);
export const customers = new SharedArray('customers', () => JSON.parse(open(TOKENS_PATH)).customers);
export const queues = new SharedArray('queues', () => JSON.parse(open(TOKENS_PATH)).queues);

// ownerFor mengembalikan akun owner dari antrian tertentu (pemilik yang berhak menekan Next dan membaca QR).
export function ownerFor(queue) {
  return owners.find((o) => o.id === queue.owner_id);
}

// queueForCustomer membagi pelanggan ke antrian secara berputar: indeks pelanggan i masuk antrian i mod jumlah antrian.
// Pembagian deterministik membuat pelanggan yang sama selalu mengamati antrian yang sama.
export function queueForCustomer(index) {
  return queues[index % queues.length];
}
