package project

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A server that sends its headers and then stalls must not hang the
// download (and with it `jcore init`) forever.
func TestDownloadZipTimesOutOnStalledServer(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PK"))
		w.(http.Flusher).Flush()
		<-release
	}))
	defer server.Close()
	defer close(release)

	orig := downloadClient
	downloadClient = &http.Client{Timeout: 200 * time.Millisecond}
	t.Cleanup(func() { downloadClient = orig })

	done := make(chan error, 1)
	go func() {
		_, err := downloadZip(server.URL)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a timeout error from a stalled download")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("downloadZip still hanging on a stalled server")
	}
}
