package requestid

import "context"

// ctxKey adalah tipe kunci khusus agar nilai di context tidak bertabrakan dengan paket lain
// (kunci bertipe privat tidak pernah sama dengan kunci paket lain, walau namanya mirip).
type ctxKey struct{}

// With mengembalikan salinan ctx yang membawa request ID, supaya lapisan bawah bisa membacanya (mis. untuk korelasi log).
func With(
	ctx context.Context,
	id string,
) context.Context {
	// simpan ID di context dengan kunci privat paket ini; context asli tidak diubah (context bersifat tak-berubah)
	return context.WithValue(ctx, ctxKey{}, id)
}

// From membaca request ID dari ctx. Mengembalikan string kosong bila tidak ada.
func From(
	ctx context.Context,
) string {
	// ambil nilai; bila belum diisi atau tipenya bukan string, hasilnya kosong (assertion dengan comma-ok tidak panic)
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
