/**
 * Menyimpan blob sebagai berkas unduhan. Hanya di web (tautan sementara dengan atribut download). Memakai
 * keberadaan `document`, bukan Platform, supaya modul ini bisa dimuat di tes Node.
 * Di perangkat native belum didukung: melempar galat berpesan jelas, supaya UI bisa menampilkannya.
 */
export function saveBlobAsFile(blob: Blob, filename: string): void {
  if (typeof document === 'undefined') {
    throw new Error('Ekspor berkas tersedia di versi web.');
  }
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  // Beri waktu browser memulai unduhan sebelum tautan dilepas.
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
