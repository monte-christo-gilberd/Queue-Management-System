package requestid

import "context"

// ctxKey adalah tipe kunci khusus agar nilai di context tidak bertabrakan dengan paket lain.
type ctxKey struct{}

// With mengembalikan salinan ctx yang membawa request ID, supaya lapisan bawah (mis. qrclient) bisa meneruskannya.
func With(
	ctx context.Context,
	id string,
) context.Context {
	// simpan ID di context dengan kunci privat paket ini
	return context.WithValue(ctx, ctxKey{}, id)
}

// From membaca request ID dari ctx. Mengembalikan string kosong bila tidak ada.
func From(
	ctx context.Context,
) string {
	// ambil nilai; bila belum diisi atau tipenya bukan string, hasilnya kosong
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
