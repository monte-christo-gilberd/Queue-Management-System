package httpserver_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"qms/internal/httpserver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// start menjalankan httpserver.Run di port acak dan mengembalikan alamat serta hasil Run.
func start(t *testing.T, h http.Handler, timeout time.Duration, onShutdown func()) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- httpserver.Run(ctx, httpserver.New(addr, h), timeout, onShutdown) }()

	require.Eventually(t, func() bool {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond)
	return addr, cancel, done
}

func TestRun_FinishesInFlightRequestOnShutdown(t *testing.T) {
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	addr, cancel, done := start(t, h, 3*time.Second, nil)

	status := make(chan int, 1)
	go func() {
		resp, err := http.Get("http://" + addr)
		if err != nil {
			status <- -1
			return
		}
		_ = resp.Body.Close()
		status <- resp.StatusCode
	}()
	<-started
	cancel()

	assert.Equal(t, http.StatusOK, <-status, "request yang sedang berjalan harus tuntas")
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Run tidak selesai")
	}
}

func TestRun_OnShutdownReleasesLongLivedConnections(t *testing.T) {
	stop := make(chan struct{})
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-stop // meniru koneksi SSE: baru selesai bila diberi sinyal
	})
	addr, cancel, done := start(t, h, 5*time.Second, func() { close(stop) })

	go func() {
		if resp, err := http.Get("http://" + addr); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	begin := time.Now()
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
		assert.Less(t, time.Since(begin), 2*time.Second, "tidak boleh menunggu sampai timeout")
	case <-time.After(5 * time.Second):
		t.Fatal("koneksi berumur panjang menahan shutdown")
	}
}

func TestRun_ForcesCloseAfterTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release // menggantung, tidak mendengar sinyal apa pun
	})
	addr, cancel, done := start(t, h, 200*time.Millisecond, nil)

	go func() {
		if resp, err := http.Get("http://" + addr); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	cancel()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(3 * time.Second):
		t.Fatal("Run harus mengembalikan error setelah timeout")
	}
}

func TestRun_ReturnsListenError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	err = httpserver.Run(context.Background(), httpserver.New(ln.Addr().String(), http.NotFoundHandler()), time.Second, nil)
	assert.Error(t, err, "port yang sudah dipakai harus melaporkan error, bukan diam")
}

func TestNew_SetsReadTimeoutsButNotWriteTimeout(t *testing.T) {
	srv := httpserver.New(":0", http.NotFoundHandler())
	assert.Equal(t, httpserver.ReadHeaderTimeout, srv.ReadHeaderTimeout)
	assert.Equal(t, httpserver.ReadTimeout, srv.ReadTimeout)
	assert.Equal(t, httpserver.IdleTimeout, srv.IdleTimeout)
	assert.Zero(t, srv.WriteTimeout, "WriteTimeout akan memutus koneksi SSE")
}
