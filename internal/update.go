// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// Update of the composition root: the one call that closes a turn.

package internal

import (
	"cmp"
	"slices"

	"github.com/genosis18m/Long-term-memory-go/internal/cap/llmops"
	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/content"
	"github.com/genosis18m/Long-term-memory-go/internal/domain"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
	"github.com/genosis18m/Long-term-memory-go/internal/scene"
	"github.com/genosis18m/Long-term-memory-go/internal/turn"
)

// outcomeEvent names the record Update writes for a turn's ending.
const outcomeEvent = "turn_outcome"

// Update closes the turn Search opened.
func (db *DB) Update(agentID uint64, end core.TurnEnd) (*core.TopicSlot, error) {
	ac, err := db.lockTurn(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()

	records := turnEndRecords(end)
	if len(records) == 0 {
		return nil, common.NewError(common.ErrInvalidQuery,
			"a turn closes with an input, an output or an outcome")
	}
	for _, in := range records {
		if err := content.ValidateAppend(in); err != nil {
			return nil, err
		}
	}
	for _, in := range records {
		if _, err := content.Append(ac, agentID, ac.Turn, in); err != nil {
			return nil, err
		}
	}
	return db.settleLocked(ac, agentID, ac.Scene, ac.Turn)
}

// turnEndRecords turns one closing call into the records it writes.
func turnEndRecords(end core.TurnEnd) []core.ArchiveSlot {
	var records []core.ArchiveSlot
	if end.Input != "" {
		records = append(records, core.ArchiveSlot{
			Kind: core.KindUtterance, Seq: core.SeqUser, Role: core.RoleUser,
			ContentType: core.ContentText, Content: end.Input, CreatedAt: end.CreatedAt,
		})
	}
	if end.Output != "" {
		records = append(records, core.ArchiveSlot{
			Kind: core.KindUtterance, Seq: core.SeqAgent, Role: core.RoleAgent,
			ContentType: core.ContentText, Content: end.Output, CreatedAt: end.CreatedAt,
		})
	}
	if end.Outcome != "" {
		records = append(records, core.ArchiveSlot{
			Kind: core.KindEvent, EventType: outcomeEvent,
			Content: end.Outcome, CreatedAt: end.CreatedAt,
		})
	}
	return records
}

// settleLocked distills one turn's utterances into its topic's keyword track.
func (db *DB) settleLocked(ac *domain.Context, agentID, sceneID, topicID uint64) (*core.TopicSlot, error) {
	slot, err := core.ReadSceneSlot(db.engine, agentID, sceneID)
	if err != nil {
		return nil, err
	}
	if err := turn.SettleTarget(sceneID, topicID, slot.TurnSeq); err != nil {
		return nil, err
	}
	utterances, err := content.Read(agentID, ac, topicID, core.KindUtterance)
	if err != nil {
		return nil, err
	}
	if len(utterances) == 0 {
		return nil, common.NewError(common.ErrInvalidQuery, "this turn holds no content to distill")
	}
	// Extract on the domain's cancellable context: a Close racing an in-flight Update cancels the LLM call
	// instead of waiting a full round-trip behind the lifecycle barrier.
	keywords, err := llmops.ExtractKeywords(ac.OpCtx, ac.LLM, content.RenderForDistill(utterances))
	if err != nil {
		return nil, common.NewError(common.ErrLLM, "distill turn", err)
	}
	if len(keywords) == 0 {
		return nil, common.NewError(common.ErrLLM, "turn distillation produced no keywords", nil)
	}
	topic, err := repo.CreateTurnTopicL2(db.engine, agentID, sceneID, topicID, keywords,
		slices.MinFunc(utterances, byCreatedAt).CreatedAt,
		slices.MaxFunc(utterances, byCreatedAt).CreatedAt)
	if err != nil {
		// A read-back or write failure keeps its own code: the host hears
		// what actually stopped the close.
		return nil, common.NewError(common.CodeOf(err), "create turn topic", err)
	}
	ac.SyncL2Meta(topic)
	db.consolidateScene(ac, sceneID)
	return topic, nil
}

func byCreatedAt(a, b core.ArchiveSlot) int {
	return cmp.Compare(a.CreatedAt, b.CreatedAt)
}

// consolidateScene keeps one scene's read surface bounded.
func (db *DB) consolidateScene(ac *domain.Context, sceneID uint64) {
	t := db.config.Defaults.SceneDreamTopicThreshold
	if t <= 0 || len(scene.SurfaceTopics(ac, sceneID)) <= t {
		return
	}
	db.triggerSceneDream(ac, sceneID)
}
