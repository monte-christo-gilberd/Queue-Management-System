package service

import "context"

// OutboxCounters menerima hasil akhir pengiriman email untuk metrik. Dipenuhi metrics.Mail.
type OutboxCounters interface {
	// IncSent dipanggil saat baris berhasil ditandai terkirim.
	IncSent()
	// IncFailed dipanggil saat baris berhasil ditandai gagal permanen.
	IncFailed()
}

// meteredStore membungkus OutboxStore dan menghitung hasil akhir, tanpa mengubah perilaku OutboxService.
type meteredStore struct {
	OutboxStore
	counters OutboxCounters
}

// NewMeteredOutboxStore membungkus store dengan penghitung metrik. Hasil dihitung hanya bila penandaan di database
// berhasil, sehingga angka sama dengan keadaan tabel outbox.
func NewMeteredOutboxStore(
	store OutboxStore,
	counters OutboxCounters,
) OutboxStore {
	// tanpa penghitung, kembalikan store apa adanya (tidak ada lapisan tambahan)
	if counters == nil {
		return store
	}
	return &meteredStore{OutboxStore: store, counters: counters}
}

// MarkSent menandai terkirim lalu menghitung bila berhasil.
func (m *meteredStore) MarkSent(
	ctx context.Context,
	id int64,
) error {
	// teruskan ke store asli; perilaku penyimpanan tidak berubah
	err := m.OutboxStore.MarkSent(ctx, id)
	// hitung hanya bila database benar-benar menyimpan status terkirim
	if err == nil {
		m.counters.IncSent()
	}
	return err
}

// MarkFailed menandai gagal permanen lalu menghitung bila berhasil.
func (m *meteredStore) MarkFailed(
	ctx context.Context,
	id int64,
	cause string,
) error {
	// teruskan ke store asli; perilaku penyimpanan tidak berubah
	err := m.OutboxStore.MarkFailed(ctx, id, cause)
	// hitung hanya bila database benar-benar menyimpan status gagal
	if err == nil {
		m.counters.IncFailed()
	}
	return err
}
