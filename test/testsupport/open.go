// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// Package testsupport provides shared helpers for integration tests that run against a real LLM
// service (DeepSeek by default).
package testsupport

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	memhop "github.com/genosis18m/Long-term-memory-go/api"
	internal "github.com/genosis18m/Long-term-memory-go/internal"
)

// LLM config environment variables.
const (
	EnvLLMKey   = "MEMHOP_TEST_LLM_KEY"
	EnvLLMURL   = "MEMHOP_TEST_LLM_URL"
	EnvLLMModel = "MEMHOP_TEST_LLM_MODEL"
)

// Defaults used when only MEMHOP_TEST_LLM_KEY is set.
const (
	defaultLLMURL   = "https://api.deepseek.com/v1/chat/completions"
	defaultLLMModel = "deepseek-chat"
)

// errNoLLMConfig is returned when neither env vars nor key_config.json provide an LLM API key.
var errNoLLMConfig = errors.New("testsupport: no LLM config: set " + EnvLLMKey +
	" or create test/testsupport/key_config.json")

func keyConfigPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "key_config.json")
}

// LoadLLMConfig fills cfg.LLM from env vars first, then key_config.json.
func LoadLLMConfig(cfg *internal.MemHopConfig) error { return loadLLMConfig(cfg) }

// loadLLMConfig fills cfg.LLM from env vars first, then key_config.json.
func loadLLMConfig(cfg *internal.MemHopConfig) error {
	if key := os.Getenv(EnvLLMKey); key != "" {
		cfg.LLM.APIKey = key
		cfg.LLM.APIURL = os.Getenv(EnvLLMURL)
		if cfg.LLM.APIURL == "" {
			cfg.LLM.APIURL = defaultLLMURL
		}
		cfg.LLM.Model = os.Getenv(EnvLLMModel)
		if cfg.LLM.Model == "" {
			cfg.LLM.Model = defaultLLMModel
		}
		cfg.LLM.TimeoutSecs = 120
		return nil
	}

	f, err := os.Open(keyConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return errNoLLMConfig
		}
		return fmt.Errorf("testsupport: read key_config.json: %w", err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&cfg.LLM); err != nil {
		return fmt.Errorf("testsupport: parse key_config.json: %w", err)
	}
	if cfg.LLM.APIKey == "" {
		return errNoLLMConfig
	}
	if cfg.LLM.TimeoutSecs <= 0 {
		cfg.LLM.TimeoutSecs = 120
	}
	return nil
}

// Handle is the test handle of the public API: an agent-domain session plus the file-level lifecycle
// methods of the underlying DB (Close / Checkpoint / IsClosed).
type Handle struct {
	*memhop.Session
	m *memhop.DB
}

func (h *Handle) Checkpoint() error { return h.m.Checkpoint() }
func (h *Handle) Close() error      { return h.m.Close() }
func (h *Handle) IsClosed() bool    { return h.m.IsClosed() }

// OpenMemHop opens a DB backed by a real LLM service in t.TempDir(); skips when the LLM config is
// missing, fatals otherwise.
func OpenMemHop(t *testing.T) *Handle {
	t.Helper()
	return open(t)
}

// OpenMemHopB is the *testing.B variant of OpenMemHop.
func OpenMemHopB(b *testing.B) *Handle {
	b.Helper()
	return open(b)
}

// OpenTurn enters the memory loop the way a host does: an empty sceneID asks for a fresh session
// (scene), a non-empty one continues it.
func (h *Handle) OpenTurn(sceneID string) (string, string, error) {
	res, err := h.Search(memhop.SearchQuery{SceneID: sceneID})
	if err != nil {
		return "", "", err
	}
	return res.Scene.SceneID, res.NewTopicID, nil
}

// CloseTurn closes one finished turn the way the task face does: Update lands the two originals on the
// slots the turn's dialogue owns and distills them into that topic's keyword track.
func (h *Handle) CloseTurn(user, agent string, ts int64) error {
	_, err := h.Update(memhop.TurnEnd{Input: user, Output: agent, CreatedAt: ts})
	return err
}

// open is the shared implementation for testing.T and testing.B.
func open(tb testing.TB) *Handle {
	cfg := &internal.MemHopConfig{
		DBPath:   filepath.Join(tb.TempDir(), "test.meh"),
		Defaults: internal.DefaultMemHopDefaults,
	}
	if err := loadLLMConfig(cfg); err != nil {
		tb.Skipf("跳过真实依赖测试: %v", err)
	}

	m, err := memhop.Open(cfg.DBPath, cfg.LLM, cfg.Defaults,
		&memhop.ProfileInput{Name: "test-primary", Role: "integration fixture"})
	if err != nil {
		tb.Fatalf("memhop.Open: %v", err)
	}
	sess, err := m.Primary()
	if err != nil {
		m.Close()
		tb.Fatalf("Primary: %v", err)
	}
	return &Handle{Session: sess, m: m}
}
