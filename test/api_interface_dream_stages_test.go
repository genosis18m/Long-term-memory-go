// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// `DreamReport.Stages` is how a host finds out which part of a consolidation pass did what, and after
// a failure which part never ran.

package test

import (
	"context"
	"path/filepath"
	"testing"

	internal "github.com/genosis18m/Long-term-memory-go/internal"
)

// dreamStages is the emission order of one full pass, and the whole vocabulary.
var dreamStages = []string{
	"l4_prune", "l5_prune", "l2_compress", "index_rebuild",
	"l1_nodes", "l1_hyperedges", "l1_rebuild", "l1_decay", "l0_distill",
}

var dreamStatuses = map[string]bool{"ok": true, "skipped": true, "cancelled": true, "error": true}

func TestInterfaceDreamReportsEveryStageInOrder(t *testing.T) {
	llm := newMockLLM(t)
	m := openMockDB(t, filepath.Join(t.TempDir(), "stages.meh"), llm.srv.URL,
		func(d *internal.MemHopDefaults) { d.DreamCompressMinTopics = 2 })
	db := newTestDB(t, m)
	defer db.Close()

	scoped := openSession(t, db)
	settleTurn(t, db, scoped, "用户要求重构登录模块", "登录模块开始重构")
	settleTurn(t, db, scoped, "继续把 token 校验挪进去", "token 校验已挪进去")

	rep, err := db.Dream(context.Background(), "")
	if err != nil {
		t.Fatalf("Dream: %v", err)
	}
	if len(rep.Stages) != len(dreamStages) {
		t.Fatalf("the pass reported %d stages, want the full %d: %+v",
			len(rep.Stages), len(dreamStages), rep.Stages)
	}
	for i, stage := range rep.Stages {
		if stage.Name != dreamStages[i] {
			t.Fatalf("stage %d is %q, want %q — the order a host reads progress off",
				i, stage.Name, dreamStages[i])
		}
		if !dreamStatuses[stage.Status] {
			t.Errorf("stage %q reports the status %q, outside the published vocabulary",
				stage.Name, stage.Status)
		}
	}
}
