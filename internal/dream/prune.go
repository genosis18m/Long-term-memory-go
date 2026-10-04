// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

package dream

import (
	"log/slog"
	"time"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/domain"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// ContentRetention is the sweep window a domain gets when its host did not configure one: Dream drops
// L4 content older than this, and plan nodes past it too.
const ContentRetention = 7 * 24 * time.Hour

// retentionWindow resolves the window for one domain: the host's configured value when it set a
// positive one, the engine default otherwise.
func retentionWindow(ac *domain.Context) time.Duration {
	if ac.Defaults != nil && ac.Defaults.ContentRetentionMs > 0 {
		return time.Duration(ac.Defaults.ContentRetentionMs) * time.Millisecond
	}
	return ContentRetention
}

// PruneContentStage drops the L4 records past the retention window, utterances and events alike, and
// counts what went away into the report — the log line is not something a host can read back.
func PruneContentStage(ac *domain.Context, agentID uint64, rep *core.DreamReport) {
	start := time.Now()
	cutoff := time.Now().Add(-retentionWindow(ac)).UnixMilli()
	dropped, err := repo.DropExpiredArchives(ac.Engine, agentID, ac.L4, cutoff)
	if err != nil {
		slog.Warn("dream: content prune failed", "agent", common.FormatHash(agentID), "err", err)
	} else if dropped > 0 {
		rep.L4RecordsPruned = dropped
		slog.Info("dream: content pruned", "agent", common.FormatHash(agentID), "records", dropped)
	}
	AppendStage(rep, "l4_prune", start, err)
}

// PrunePlanStage sweeps plan nodes past the window.
func PrunePlanStage(ac *domain.Context, agentID uint64, rep *core.DreamReport) {
	start := time.Now()
	cutoff := time.Now().Add(-retentionWindow(ac)).UnixMilli()
	type sweep struct {
		topicID uint64
		ids     []uint64
	}
	var sweeps []sweep
	var doomed []uint64
	// The engine is read rather than the cache: Dream is a disk maintainer, not a
	// hot path, and the sweep must not be shaped by a cache that could be behind.
	aggs, err := repo.CollectPlanNodes(ac.Engine, agentID)
	if err != nil {
		// The exemption is per-tree and derived from every node in it, so an unreadable node is exactly the
		// one that could still be holding a tree alive.
		slog.Warn("dream: plan nodes not swept", "agent", common.FormatHash(agentID), "err", err)
		AppendStage(rep, "l5_prune", start, err)
		return
	}
	for _, agg := range aggs {
		if agg.HasNonDone && agg.LastActiveAt >= cutoff {
			continue
		}
		var ids []uint64
		for _, n := range agg.Nodes {
			if n.UpdatedAt < cutoff {
				ids = append(ids, n.IDHash)
			}
		}
		if len(ids) == 0 {
			continue
		}
		sweeps = append(sweeps, sweep{topicID: agg.TopicID, ids: ids})
		doomed = append(doomed, ids...)
	}
	if len(doomed) > 0 {
		if err = repo.DeletePlanNodesByIDs(ac.Engine, agentID, doomed); err != nil {
			slog.Warn("dream: plan-node prune failed", "agent", common.FormatHash(agentID), "err", err)
		} else {
			rep.L5NodesPruned = len(doomed)
			for _, s := range sweeps {
				ac.Plans.RemoveNodes(s.topicID, s.ids)
			}
			slog.Info("dream: plan nodes pruned", "agent", common.FormatHash(agentID), "nodes", len(doomed))
		}
	}
	AppendStage(rep, "l5_prune", start, err)
}
