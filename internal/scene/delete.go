// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

package scene

import (
	"errors"

	"github.com/genosis18m/Long-term-memory-go/internal/domain"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// DeleteCascade removes the given L2 records — scene slots and/or topics — with the L4 content and L5
// plan trees they own and their cache entries.
func DeleteCascade(ac *domain.Context, agentID uint64, scenes, topics []uint64) error {
	planNodes, err := repo.PlanNodeIDsByTopicIDs(ac.Engine, agentID, topics)
	if err != nil {
		return err
	}
	if err := repo.DeleteTopicArchives(ac.Engine, agentID, ac.L4, topics); err != nil {
		return err
	}
	if err := repo.DeletePlanNodesByIDs(ac.Engine, agentID, planNodes); err != nil {
		return err
	}
	records := make([]uint64, 0, len(scenes)+len(topics))
	records = append(records, topics...)
	records = append(records, scenes...)
	if err := repo.DeleteL2Records(ac.Engine, agentID, records); err != nil {
		return err
	}
	ac.RemoveTopicsFromIndices(topics)
	return nil
}

// DetachGraph clears the L3 anchor of every scene that named graphID, scanning the domain because
// anchors live only on scenes — a graph slot keeps no reverse list.
func DetachGraph(engine *core.StorageEngine, agentID uint64, graphID uint64) error {
	scenes, err := repo.CollectAllScenesL2(engine, agentID)
	if err != nil {
		return err
	}
	var errs []error
	for _, slot := range scenes {
		if slot.L3ID != graphID {
			continue
		}
		slot.L3ID = 0
		if err := core.WriteSceneSlot(engine, agentID, slot.SceneID, &slot); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
