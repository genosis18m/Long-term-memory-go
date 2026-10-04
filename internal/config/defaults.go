// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

package config

import (
	"fmt"
	"math"
	"time"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
)

// MemHopDefaults holds the tuning knobs a caller supplies: the consolidation thresholds and the
// idle-domain TTL.
type MemHopDefaults struct {
	// SceneDreamTopicThreshold is how many depth-1 topics one scene may
	// accumulate before its Dream is scheduled. Negative disables the trigger.
	SceneDreamTopicThreshold int `json:"scene_dream_topic_threshold"`
	// DreamCompressMinTopics is the smallest depth-1 topic count a Dream pass compresses; below it a scene
	// keeps raw detail.
	DreamCompressMinTopics int `json:"dream_compress_min_topics"`
	// AgentIdleTTLMs reclaims an idle agent's in-memory contexts. Negative disables
	// the reclaim.
	AgentIdleTTLMs int64 `json:"agent_idle_ttl_ms"`
	// ContentRetentionMs is how long a turn's records (L4 content and L5 plan nodes) outlive it before a
	// Dream sweeps them.
	ContentRetentionMs int64 `json:"content_retention_ms"`
}

// MaxContentRetentionMs is the largest window the sweep can represent: one whose millisecond count
// still fits a time.Duration when scaled to nanoseconds.
const MaxContentRetentionMs = int64(math.MaxInt64 / int64(time.Millisecond))

// Validate refuses a retention window the engine cannot honour.
func (m MemHopDefaults) Validate() error {
	switch {
	case m.ContentRetentionMs < 0:
		return common.NewError(common.ErrConfig, fmt.Sprintf(
			"content_retention_ms %d asks to switch the sweep off, which the engine does not offer: retention is what bounds the file, so leave it 0 for the default (%d ms) or name a window you mean to keep",
			m.ContentRetentionMs, DefaultMemHopDefaults.ContentRetentionMs))
	case m.ContentRetentionMs > MaxContentRetentionMs:
		return common.NewError(common.ErrConfig, fmt.Sprintf(
			"content_retention_ms %d is past the longest window the sweep can measure (%d ms): a longer one wraps the duration around and puts the cutoff in the future, which reads every record in the domain as expired",
			m.ContentRetentionMs, MaxContentRetentionMs))
	}
	return nil
}

// DefaultMemHopDefaults is the single hardcoded source of engine defaults.
var DefaultMemHopDefaults = MemHopDefaults{
	SceneDreamTopicThreshold: 24,
	DreamCompressMinTopics:   20,
	AgentIdleTTLMs:           3600000,                 // 60 minutes
	ContentRetentionMs:       7 * 24 * 60 * 60 * 1000, // seven days
}

// Normalized reads a caller's struct the way the vocabulary above says to read it: an unfilled knob
// (0) takes the library default, a negative takes the off spelling.
func (m MemHopDefaults) Normalized() MemHopDefaults {
	out := m
	if out.SceneDreamTopicThreshold == 0 {
		out.SceneDreamTopicThreshold = DefaultMemHopDefaults.SceneDreamTopicThreshold
	}
	switch {
	case out.DreamCompressMinTopics == 0:
		out.DreamCompressMinTopics = DefaultMemHopDefaults.DreamCompressMinTopics
	case out.DreamCompressMinTopics < 0:
		out.DreamCompressMinTopics = 0
	}
	if out.AgentIdleTTLMs == 0 {
		out.AgentIdleTTLMs = DefaultMemHopDefaults.AgentIdleTTLMs
	}
	if out.ContentRetentionMs <= 0 {
		out.ContentRetentionMs = DefaultMemHopDefaults.ContentRetentionMs
	}
	return out
}
