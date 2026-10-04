// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// TimeoutSecs exists for one endpoint shape: a server that takes the connection and is slower to
// answer than the caller can wait.

package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/config"
)

func TestSlowEndpointIsAbandonedInsideItsOwnWindow(t *testing.T) {
	// The endpoint answers after twice the window the caller named.
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		attempts.Add(1)
		time.Sleep(2 * time.Second)
		// A reply that would have been usable had it arrived in time: with no window, this call returns
		// content instead of an error, which is the outcome the first assertion below refuses.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"keywords\":[\"late\"]}"}}]}`))
	}))
	t.Cleanup(srv.Close)

	provider := New(config.LlmConfig{APIURL: srv.URL, APIKey: "test", Model: "m", TimeoutSecs: 1})
	start := time.Now()
	_, err := provider.Chat(context.Background(), "sys", "user", 512)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("an endpoint that answered after %s came back as a success", elapsed)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("a 1s window took %s to give up: the timeout is either not honoured or repeated per attempt", elapsed)
	}
	if code := common.CodeOf(err); code != common.ErrLLM {
		t.Fatalf("an endpoint that never answered in time is reported as code %d (%v), want ErrLLM — "+
			"a host reading ErrCancelled goes to look at its own context instead of at the service", code, err)
	}
	if got := attempts.Load(); got > 1 {
		t.Fatalf("the window ran %d times: every repeat holds this domain's lock open for another "+
			"window, which is exactly what the caller named a timeout to avoid", got)
	}
}
