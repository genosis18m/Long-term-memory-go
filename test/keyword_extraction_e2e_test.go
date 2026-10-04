// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

//go:build integration

// Real-LLM regression tests for the keyword-extraction non-JSON bug (meowagent
// docs/issues/memhop_search_keywords_nonjson.md).

package test

import (
	"context"
	"testing"
	"time"

	memhop "github.com/genosis18m/Long-term-memory-go/api"
	internal "github.com/genosis18m/Long-term-memory-go/internal"
	"github.com/genosis18m/Long-term-memory-go/internal/cap/llmops"
	"github.com/genosis18m/Long-term-memory-go/internal/llm"
	"github.com/genosis18m/Long-term-memory-go/test/testsupport"
)

// longSessionText renders one full Locomo session as "speaker: text" lines, mirroring the host's
// whole-session injection that triggered the bug.
func longSessionText(t *testing.T, sessionIdx int) string {
	t.Helper()
	items := loadLocomo10(t, 1)
	sessions := items[0].Sessions
	if sessionIdx >= len(sessions) {
		t.Fatalf("fixture has %d sessions, want idx %d", len(sessions), sessionIdx)
	}
	var sb []byte
	for _, tn := range sessions[sessionIdx].Turns {
		sb = append(sb, tn.Speaker...)
		sb = append(sb, ": "...)
		sb = append(sb, tn.Text...)
		sb = append(sb, '\n')
	}
	return string(sb)
}

// TestExtractKeywordsLongInputRealLLM reproduces the original failure mode: a >2000-char whole-session
// text handed to the real LLM.
func TestExtractKeywordsLongInputRealLLM(t *testing.T) {
	cfg := &internal.MemHopConfig{}
	if err := testsupport.LoadLLMConfig(cfg); err != nil {
		t.Skipf("LLM not configured: %v", err)
	}
	p := llm.New(cfg.LLM)
	// Session 2 is 4322 chars (> keywordChunkRunes → chunked path).
	longText := longSessionText(t, 2)
	// Session 0 is 1560 chars (single-pass path with format retry).
	midText := longSessionText(t, 0)
	for name, text := range map[string]string{"chunked": longText, "single": midText} {
		t.Run(name, func(t *testing.T) {
			// Sampling variance: several attempts raise the chance of hitting
			// the summary-output form the bug report observed.
			for i := 0; i < 3; i++ {
				kw, err := llmops.ExtractKeywords(context.Background(), p, text)
				if err != nil {
					t.Fatalf("attempt %d: ExtractKeywords returned error: %v", i, err)
				}
				if len(kw) == 0 {
					t.Fatalf("attempt %d: no keywords at all", i)
				}
			}
		})
	}
}

// TestUpdateLongTurnSettles runs the write chain (real LLM) with whole-session long inputs.
func TestUpdateLongTurnSettles(t *testing.T) {
	db := testsupport.OpenMemHop(t)
	defer db.Close()

	texts := []string{longSessionText(t, 2), longSessionText(t, 0)}
	sceneID, _, err := db.OpenTurn("")
	if err != nil {
		t.Fatalf("OpenTurn: %v", err)
	}
	base := time.Now().UnixMilli()
	for i, text := range texts {
		ts := base + int64(i)*1000
		if _, _, err := db.OpenTurn(sceneID); err != nil {
			t.Fatalf("OpenTurn %d: %v", i, err)
		}
		if err := db.CloseTurn(text, "已了解这段长对话", ts); err != nil {
			// Failing loudly is allowed; settling a degraded turn is not.
			res, rerr := db.Search(memhop.SearchQuery{SceneID: sceneID})
			if rerr != nil {
				t.Fatalf("attempt %d: the close failed (%v) and the scene cannot be read: %v", i, err, rerr)
			}
			if len(res.Topics) != i {
				t.Fatalf("attempt %d: the close failed (%v) but settled %d topics, want %d", i, err, len(res.Topics), i)
			}
			t.Logf("attempt %d: the close was refused without degrading the turn: %v", i, err)
			continue
		}
	}
	res, err := db.Search(memhop.SearchQuery{SceneID: sceneID})
	if err != nil {
		t.Fatalf("session read: %v", err)
	}
	if len(res.Topics) != len(texts) {
		t.Fatalf("surface = %d topics, want %d", len(res.Topics), len(texts))
	}
	for _, tp := range res.Topics {
		if len(tp.FusedKeywords) == 0 {
			t.Errorf("long turn %s settled with no keywords", tp.ID)
		}
		id := tp.ID
		if owned, err := db.SearchL4(memhop.L4Query{TopicID: &id}); err != nil || len(owned) != 2 {
			t.Errorf("long turn %s lost its originals: %d owned, err %v", tp.ID, len(owned), err)
		}
	}
}

// TestExtractKeywordsLongInput pins the capability contract directly: a long rendered transcript must
// never surface an error, and must yield keywords.
func TestExtractKeywordsLongInput(t *testing.T) {
	cfg := &internal.MemHopConfig{}
	if err := testsupport.LoadLLMConfig(cfg); err != nil {
		t.Skipf("LLM not configured: %v", err)
	}
	p := llm.New(cfg.LLM)
	kw, err := llmops.ExtractKeywords(context.Background(), p, "User: "+longSessionText(t, 2)+"\nAssistant: 收到")
	if err != nil {
		t.Fatalf("ExtractKeywords: %v", err)
	}
	if len(kw) == 0 {
		t.Fatal("long turn distilled to nothing")
	}
}
