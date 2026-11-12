package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// find mengembalikan metrik bernama name yang punya semua label yang diminta, atau nil bila tidak ada.
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
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			got := map[string]string{}
			for _, l := range m.GetLabel() {
				got[l.GetName()] = l.GetValue()
			}
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
	return nil
}

// counter membaca nilai counter; 0 bila deret belum ada.
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

// allLabelValues mengumpulkan semua nilai label dari semua metrik, untuk memeriksa tidak ada rahasia yang bocor.
func allLabelValues(
	t *testing.T,
	reg *prometheus.Registry,
) []string {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var out []string
	for _, f := range families {
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				out = append(out, l.GetValue())
			}
		}
	}
	return out
}
