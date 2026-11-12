package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
)

// writeFixture membuat kunci uji dan tiga CSV masukan di folder sementara, lalu mengembalikan folder, path kunci, dan
// kunci publik untuk memverifikasi hasilnya.
func writeFixture(t *testing.T) (dir, keyPath string, pub *rsa.PublicKey) {
	t.Helper()
	dir = t.TempDir()
	// kunci RSA 2048 dalam format PKCS#8, sama dengan keluaran `openssl genpkey` di scripts/gen-auth-keys.sh
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPath = filepath.Join(dir, "auth-private.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	// CSV: 2 owner (id 1 dan 2), 3 pelanggan (id 3 sampai 5), 2 antrian milik owner 1 dan 2
	files := map[string]string{"owners.csv": "1\n2\n", "customers.csv": "3\n4\n5\n", "queues.csv": "10,1\n11,2\n"}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, keyPath, &key.PublicKey
}

// Token yang dibuat harus lolos verifikasi seperti di backend: RS256 dengan kunci publik, klaim sub berupa teks id,
// iat dan exp terisi, dan header kid sesuai.
func TestRun_TokensVerifyLikeBackend(t *testing.T) {
	dir, keyPath, pub := writeFixture(t)
	// satu menit di masa lalu: token langsung sah terhadap jam sekarang, seperti token yang baru diterbitkan
	now := time.Now().Add(-time.Minute).Truncate(time.Second)

	if err := run(keyPath, dir, "qms-1", 24*time.Hour, now); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	var out Output
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	// jumlah akun dan antrian sesuai CSV
	if len(out.Owners) != 2 || len(out.Customers) != 3 || len(out.Queues) != 2 {
		t.Fatalf("jumlah tidak sesuai: %d owner, %d pelanggan, %d antrian", len(out.Owners), len(out.Customers), len(out.Queues))
	}
	// pemetaan antrian ke owner dipertahankan
	if out.Queues[1].ID != 11 || out.Queues[1].OwnerID != 2 {
		t.Fatalf("pemetaan antrian salah: %+v", out.Queues[1])
	}
	// tiap token diverifikasi dengan cara yang sama seperti backend (hanya RS256, kunci publik)
	for _, acct := range append(append([]Account{}, out.Owners...), out.Customers...) {
		claims := jwt.MapClaims{}
		tok, err := jwt.ParseWithClaims(acct.Token, claims, func(tk *jwt.Token) (interface{}, error) {
			if tk.Method != jwt.SigningMethodRS256 {
				t.Fatalf("metode tanda tangan %v, harus RS256", tk.Method.Alg())
			}
			return pub, nil
		})
		// tanda tangan, iat, dan exp semuanya harus sah terhadap jam sekarang
		if err != nil || !tok.Valid {
			t.Fatalf("token id %d tidak terverifikasi: %v", acct.ID, err)
		}
		if claims["sub"] != strconv.FormatInt(acct.ID, 10) {
			t.Fatalf("sub = %v, diharapkan %d", claims["sub"], acct.ID)
		}
		if int64(claims["iat"].(float64)) != now.Unix() || int64(claims["exp"].(float64)) != now.Add(24*time.Hour).Unix() {
			t.Fatalf("iat/exp salah: %v", claims)
		}
		if tok.Header["kid"] != "qms-1" {
			t.Fatalf("kid = %v", tok.Header["kid"])
		}
	}
}

// Masukan kosong atau rusak harus gagal dengan pesan jelas, bukan menulis berkas kosong.
func TestRun_RejectsEmptyOrBadInput(t *testing.T) {
	dir, keyPath, _ := writeFixture(t)
	// owners.csv kosong
	if err := os.WriteFile(filepath.Join(dir, "owners.csv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(keyPath, dir, "qms-1", time.Hour, time.Now()); err == nil {
		t.Fatal("masukan kosong seharusnya gagal")
	}
	// id bukan angka
	if err := os.WriteFile(filepath.Join(dir, "owners.csv"), []byte("abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(keyPath, dir, "qms-1", time.Hour, time.Now()); err == nil {
		t.Fatal("id bukan angka seharusnya gagal")
	}
	// kunci bukan PEM RSA
	bad := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(bad, []byte("bukan pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(bad, dir, "qms-1", time.Hour, time.Now()); err == nil {
		t.Fatal("kunci rusak seharusnya gagal")
	}
}
