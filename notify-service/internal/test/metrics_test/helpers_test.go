package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// find mengembalikan metrik bernama name yang punya semua label yang diminta, atau nil bila tidak ada.
// Dipakai tes untuk membaca nilai metrik tanpa menjalankan server /metrics.
func find(
	t *testing.T,
	reg *prometheus.Registry,
	name string,
	labels map[string]string,
) *dto.Metric {
	t.Helper()
	// Gather mengembalikan metrik yang berhasil dikumpulkan walau satu collector gagal (promhttp dengan
	// ContinueOnError menyajikan hal yang sama), jadi galatnya tidak fatal di sini.
	families, _ := reg.Gather()
	for _, f := range families {
		// lewati keluarga metrik dengan nama lain
		if f.GetName() != name {
			continue
		}
		// periksa tiap deret (kombinasi label) di keluarga ini
		for _, m := range f.GetMetric() {
			// ubah daftar label deret menjadi peta nama -> nilai
			got := map[string]string{}
			for _, l := range m.GetLabel() {
				got[l.GetName()] = l.GetValue()
			}
			// deret cocok bila semua label yang diminta sama persis
			match := true
			for k, v := range labels {
				if got[k] != v {
					match = false
				}
			}
			if match {
				return m
			}
		}
	}
	// tidak ada deret yang cocok
	return nil
}

// counter membaca nilai counter; 0 bila deret belum ada (counter yang belum pernah naik belum muncul).
func counter(
	t *testing.T,
	reg *prometheus.Registry,
	name string,
	labels map[string]string,
) float64 {
	t.Helper()
	if m := find(t, reg, name, labels); m != nil {
		return m.GetCounter().GetValue()
	}
	return 0
}
