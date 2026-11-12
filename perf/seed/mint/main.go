// Command mint membuat token login (JWT RS256) untuk akun seed uji beban, tanpa login nyata.
//
// Registrasi dan login memakai bcrypt, terlalu lambat untuk ribuan sampai jutaan akun. Di lingkungan uji, token dibuat
// langsung dengan KUNCI UJI (keys-perf/auth-private.pem, tidak pernah dipakai di prod) dan backend uji memverifikasinya
// dengan kunci publik pasangannya, persis seperti token buatan auth-service (klaim sub, iat, exp, header kid).
//
// Masukan: tiga berkas CSV dari database uji, dibuat oleh `make perf-seed`:
//
//	owners.csv     satu kolom: id user owner
//	customers.csv  satu kolom: id user pelanggan
//	queues.csv     dua kolom: id antrian, id owner
//
// Keluaran: satu berkas JSON (bawaan perf/data/tokens.json) yang dibaca skrip k6 dan penahan SSE:
//
//	{"owners":[{"id":1,"token":"..."}], "customers":[...], "queues":[{"id":1,"owner_id":1}], "issued_at":..., "expires_at":...}
//
// Pemakaian:
//
//	go run . -key ../../../keys-perf/auth-private.pem -dir ../../data
package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt"
)

// Account adalah satu akun beserta token login-nya.
type Account struct {
	ID    int64  `json:"id"`
	Token string `json:"token"`
}

// Queue memetakan antrian ke owner-nya (owner yang berhak menekan Next dan membaca QR).
type Queue struct {
	ID      int64 `json:"id"`
	OwnerID int64 `json:"owner_id"`
}

// Output adalah isi tokens.json.
type Output struct {
	Owners    []Account `json:"owners"`
	Customers []Account `json:"customers"`
	Queues    []Queue   `json:"queues"`
	IssuedAt  int64     `json:"issued_at"`
	ExpiresAt int64     `json:"expires_at"`
}

func main() {
	// semua pengaturan lewat flag supaya skrip Makefile mudah memanggilnya
	keyPath := flag.String("key", "keys-perf/auth-private.pem", "kunci privat RSA uji (PEM PKCS#8)")
	dir := flag.String("dir", "perf/data", "folder berisi owners.csv, customers.csv, queues.csv dan tempat tokens.json ditulis")
	kid := flag.String("kid", "qms-1", "nilai header kid (harus sama dengan AUTH_JWT_KID auth-service uji)")
	ttl := flag.Duration("ttl", 24*time.Hour, "masa berlaku token (sama dengan token login asli: 24 jam)")
	flag.Parse()

	if err := run(*keyPath, *dir, *kid, *ttl, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "mint:", err)
		os.Exit(1)
	}
}

// run membaca CSV, menandatangani token tiap akun, dan menulis tokens.json. now diberikan agar bisa diuji.
func run(keyPath, dir, kid string, ttl time.Duration, now time.Time) error {
	// kunci privat uji
	pemBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("baca kunci: %w", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(pemBytes)
	if err != nil {
		return fmt.Errorf("parse kunci (harus PEM RSA): %w", err)
	}
	// daftar id dari CSV hasil query database uji
	ownerIDs, err := readIDs(filepath.Join(dir, "owners.csv"))
	if err != nil {
		return err
	}
	customerIDs, err := readIDs(filepath.Join(dir, "customers.csv"))
	if err != nil {
		return err
	}
	queues, err := readQueues(filepath.Join(dir, "queues.csv"))
	if err != nil {
		return err
	}
	if len(ownerIDs) == 0 || len(customerIDs) == 0 || len(queues) == 0 {
		return errors.New("owners, customers, dan queues tidak boleh kosong (jalankan make perf-seed dulu)")
	}
	out := Output{IssuedAt: now.Unix(), ExpiresAt: now.Add(ttl).Unix(), Queues: queues}
	// tanda tangani satu token per akun
	if out.Owners, err = sign(key, kid, ownerIDs, now, ttl); err != nil {
		return err
	}
	if out.Customers, err = sign(key, kid, customerIDs, now, ttl); err != nil {
		return err
	}
	// tulis dengan izin terbatas: berkas ini berisi token yang valid selama masa berlakunya
	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}
	dest := filepath.Join(dir, "tokens.json")
	if err := os.WriteFile(dest, raw, 0o600); err != nil {
		return err
	}
	fmt.Printf("tokens.json: %d owner, %d pelanggan, %d antrian (berlaku sampai %s)\n",
		len(out.Owners), len(out.Customers), len(out.Queues), time.Unix(out.ExpiresAt, 0).Format(time.RFC3339))
	return nil
}

// sign membuat token RS256 untuk setiap id: klaim sub (id sebagai teks), iat, exp, dan header kid, sama dengan
// auth-service (internal/service/token_issuer.go).
func sign(key interface{}, kid string, ids []int64, now time.Time, ttl time.Duration) ([]Account, error) {
	out := make([]Account, 0, len(ids))
	for _, id := range ids {
		t := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub": strconv.FormatInt(id, 10),
			"iat": now.Unix(),
			"exp": now.Add(ttl).Unix(),
		})
		t.Header["kid"] = kid
		s, err := t.SignedString(key)
		if err != nil {
			return nil, fmt.Errorf("tanda tangan id %d: %w", id, err)
		}
		out = append(out, Account{ID: id, Token: s})
	}
	return out, nil
}

// readIDs membaca berkas CSV satu kolom berisi id.
func readIDs(path string) ([]int64, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for i, r := range rows {
		id, err := strconv.ParseInt(r[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s baris %d: %w", path, i+1, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// readQueues membaca berkas CSV dua kolom: id antrian dan id owner.
func readQueues(path string) ([]Queue, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	qs := make([]Queue, 0, len(rows))
	for i, r := range rows {
		if len(r) < 2 {
			return nil, fmt.Errorf("%s baris %d: butuh dua kolom", path, i+1)
		}
		qid, err1 := strconv.ParseInt(r[0], 10, 64)
		oid, err2 := strconv.ParseInt(r[1], 10, 64)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("%s baris %d: id bukan angka", path, i+1)
		}
		qs = append(qs, Queue{ID: qid, OwnerID: oid})
	}
	return qs, nil
}

// readCSV membaca semua baris tidak kosong dari berkas CSV tanpa header.
func readCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var out [][]string
	for _, row := range rows {
		if len(row) > 0 && row[0] != "" {
			out = append(out, row)
		}
	}
	return out, nil
}
