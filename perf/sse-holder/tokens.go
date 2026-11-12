package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// Account adalah satu akun dengan token login (isi tokens.json dari perf/seed/mint).
type Account struct {
	ID    int64  `json:"id"`
	Token string `json:"token"`
}

// Queue memetakan antrian ke owner-nya.
type Queue struct {
	ID      int64 `json:"id"`
	OwnerID int64 `json:"owner_id"`
}

// Tokens adalah isi tokens.json.
type Tokens struct {
	Owners    []Account `json:"owners"`
	Customers []Account `json:"customers"`
	Queues    []Queue   `json:"queues"`
}

// loadTokens membaca dan memeriksa tokens.json.
func loadTokens(path string) (*Tokens, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("baca %s (jalankan make perf-seed dulu): %w", path, err)
	}
	var t Tokens
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(t.Owners) == 0 || len(t.Customers) == 0 || len(t.Queues) == 0 {
		return nil, fmt.Errorf("%s kosong: owner, pelanggan, atau antrian tidak ada", path)
	}
	return &t, nil
}

// owner mencari akun owner berdasarkan id user; nil bila tidak ada.
func (t *Tokens) owner(id int64) *Account {
	for i := range t.Owners {
		if t.Owners[i].ID == id {
			return &t.Owners[i]
		}
	}
	return nil
}

// hardBlocked adalah domain yang SELALU ditolak: produksi.
var hardBlocked = []string{"montechristo.my.id"}

// privateHost cocok dengan localhost, loopback, rentang privat RFC 1918, link-local, dan akhiran nama lokal.
var privateHost = regexp.MustCompile(`^(localhost|\[::1\]|127\..*|10\..*|192\.168\..*|169\.254\..*|172\.(1[6-9]|2[0-9]|3[01])\..*|.*\.(local|test|internal|localhost))$`)

// assertSafeTarget menolak target produksi atau non-privat (kecuali allowPublic), seperti assertSafeTarget di k6
// (perf/k6/lib/config.js). blockedExtra = daftar host tambahan dipisah koma (PERF_BLOCKED_HOSTS).
func assertSafeTarget(base string, allowPublic bool, blockedExtra string) error {
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("base URL tidak valid: %q", base)
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(u.Host, "[") {
		host = "[" + host + "]"
	}
	blocked := append([]string{}, hardBlocked...)
	for _, b := range strings.Split(blockedExtra, ",") {
		if b = strings.TrimSpace(strings.ToLower(b)); b != "" {
			blocked = append(blocked, b)
		}
	}
	for _, b := range blocked {
		if host == b || strings.HasSuffix(host, "."+b) {
			return fmt.Errorf("target %s cocok dengan host terlarang (%s): uji beban tidak boleh menembak produksi", base, b)
		}
	}
	if !privateHost.MatchString(host) && !allowPublic {
		return fmt.Errorf("target %s bukan jaringan privat; bila ini memang mesin uji, pakai -allow-public-target", base)
	}
	return nil
}
