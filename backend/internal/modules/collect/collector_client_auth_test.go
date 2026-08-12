package collect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCollectorClientSendsInternalToken(t *testing.T) {
	t.Parallel()

	const token = "collector-internal-token-32-chars!"
	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Get(collectorInternalTokenHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
	}))
	defer server.Close()

	client := NewCollectorClient(server.URL, time.Second, token)
	if _, err := client.FetchProviders(context.Background()); err != nil {
		t.Fatalf("FetchProviders: %v", err)
	}
	if got := <-received; got != token {
		t.Fatalf("internal token header = %q, want configured token", got)
	}
}
