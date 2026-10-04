// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// L2 scene record primitives plus topic compression planning.
package repo

import (
	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// MaxDepth: topic depth threshold that triggers deletion on sinking.
const MaxDepth = 4

// CompressTopicsL2 sinks every listed topic one level deeper under parentID; topics reaching MaxDepth
// are deleted instead of rewritten.
func CompressTopicsL2(engine *core.StorageEngine, agentID uint64, ids []uint64, parentID uint64) error {
	var writes []core.RecordEntry
	var deletes []uint64
	for _, id := range ids {
		topic, err := core.ReadTopicLenient(engine, agentID, id)
		switch {
		case err != nil && common.CodeOf(err) == common.ErrNotFound:
			continue // already gone: this member just drops out of the group
		case err != nil:
			// The fused parent is already on disk: skipping an unreadable member would show both the group's
			// summary and its originals, so the failure is the caller's to roll back, not to swallow.
			return common.NewError(common.ErrIO, "read topic to sink", err)
		case topic == nil:
			// The ids come from the topic listing, so one naming a foreign record
			// is the cache and the disk disagreeing.
			return common.NewError(common.ErrIO, common.FormatHash(id)+" names no topic record", nil)
		}
		topic.Depth++
		topic.ParentID = &parentID
		if topic.Depth >= MaxDepth {
			deletes = append(deletes, topic.ID)
			continue
		}
		entry, err := core.TopicEntry(agentID, topic)
		if err != nil {
			return err
		}
		writes = append(writes, entry)
	}
	if _, err := engine.WriteRecordBatch(writes); err != nil {
		return err
	}
	if _, err := engine.DeleteRecordBatch(agentID, deletes); err != nil {
		return err
	}
	return nil
}

// RestoreSunkTopicsL2 undoes one sink: every listed topic that now hangs on parentID comes back up a
// level with no parent.
func RestoreSunkTopicsL2(engine *core.StorageEngine, agentID uint64, ids []uint64, parentID uint64) error {
	for _, id := range ids {
		topic, err := core.ReadTopicLenient(engine, agentID, id)
		switch {
		case err != nil && common.CodeOf(err) == common.ErrNotFound:
			continue // this member is gone; there is nothing to bring back
		case err != nil:
			return common.NewError(common.ErrIO, "read topic to restore", err)
		case topic == nil || topic.ParentID == nil || *topic.ParentID != parentID:
			continue // never sunk under this parent: not this group's to undo
		}
		if topic.Depth > 1 {
			topic.Depth--
		}
		topic.ParentID = nil
		if err := core.WriteTopicSlot(engine, agentID, topic.ID, topic); err != nil {
			return err
		}
	}
	return nil
}

// TopicIDsBySceneL2 enumerates every topic (any depth) owned by one of the given scenes.
func TopicIDsBySceneL2(engine *core.StorageEngine, agentID uint64, sceneIDs ...uint64) ([]uint64, error) {
	topics, err := core.CollectAllTopicsStrict(engine, agentID)
	if err != nil {
		return nil, err
	}
	set := common.ToSet(sceneIDs)
	var ids []uint64
	for _, topic := range topics {
		if _, ok := set[topic.SceneID]; ok {
			ids = append(ids, topic.ID)
		}
	}
	return ids, nil
}

// DeleteL2Records tombstones the given L2 ids — scene slots and topics in any mix — and reads nothing.
func DeleteL2Records(engine *core.StorageEngine, agentID uint64, ids []uint64) error {
	_, err := engine.DeleteRecordBatch(agentID, ids)
	return err
}

// MergeScenesL2 rewrites topics of the secondary scenes to the primary scene in one batch, then
// deletes the secondary scene records (now empty).
func MergeScenesL2(engine *core.StorageEngine, agentID uint64, primaryID uint64, secondaryIDs []uint64) error {
	topics, err := core.CollectAllTopicsStrict(engine, agentID)
	if err != nil {
		return err
	}
	secondarySet := common.ToSet(secondaryIDs)
	var writes []core.RecordEntry
	for _, topic := range topics {
		if _, ok := secondarySet[topic.SceneID]; !ok {
			continue
		}
		topic.SceneID = primaryID
		entry, err := core.TopicEntry(agentID, &topic)
		if err != nil {
			return err
		}
		writes = append(writes, entry)
	}
	if _, err := engine.WriteRecordBatch(writes); err != nil {
		return err
	}
	return DeleteL2Records(engine, agentID, secondaryIDs)
}

// OpenSceneTurn bumps the scene's turn counter and returns the updated record.
func OpenSceneTurn(engine *core.StorageEngine, agentID uint64, sceneID uint64, stamp int64) (*core.SceneSlot, error) {
	slot, err := core.ReadSceneSlot(engine, agentID, sceneID)
	if err != nil {
		return nil, err
	}
	slot.TurnSeq++
	slot.LastUsedAt = stamp
	if err := core.WriteSceneSlot(engine, agentID, sceneID, slot); err != nil {
		return nil, err
	}
	return slot, nil
}

// listByID reads each id through get: an id that names no record is skipped (a missing record selects
// nothing), while a record that will not read back stops the listing.
func listByID[T any](engine *core.StorageEngine, agentID uint64, ids []uint64, get func(uint64) (*T, error)) ([]T, error) {
	var out []T
	for _, id := range ids {
		v, err := get(id)
		if err != nil {
			if common.CodeOf(err) == common.ErrNotFound {
				continue
			}
			return nil, err
		}
		out = append(out, *v)
	}
	return out, nil
}

// ListScenesL2 reads the named scenes with the skip-missing listing rule.
func ListScenesL2(engine *core.StorageEngine, agentID uint64, ids []uint64) ([]core.SceneSlot, error) {
	return listByID(engine, agentID, ids, func(id uint64) (*core.SceneSlot, error) {
		return core.ReadSceneSlot(engine, agentID, id)
	})
}

// CreateSceneL2 stores a scene record built by the caller.
func CreateSceneL2(engine *core.StorageEngine, agentID uint64, slot *core.SceneSlot) error {
	_, err := core.ReadSceneSlot(engine, agentID, slot.SceneID)
	if err == nil {
		return nil
	}
	if common.CodeOf(err) != common.ErrNotFound {
		// "Not there" and "will not read" differ, and only the first may be written over: the unreadable
		// record still holds the turn counter that mints this domain's turn ids, and a fresh record resets it.
		return common.NewError(common.CodeOf(err), "create scene: the record under that id will not read", err)
	}
	return core.WriteSceneSlot(engine, agentID, slot.SceneID, slot)
}

// CollectAllScenesL2 returns every scene record of the agent domain; the scan is strict.
func CollectAllScenesL2(engine *core.StorageEngine, agentID uint64) ([]core.SceneSlot, error) {
	return core.CollectAllStrict[core.SceneSlot](engine, agentID, core.RecL2Scene)
}

// TopicClosureL2 gathers a topic and its recursive children (any depth); the result is empty when the
// root topic does not exist.
func TopicClosureL2(engine *core.StorageEngine, agentID uint64, root uint64) ([]uint64, error) {
	topics, err := core.CollectAllTopicsStrict(engine, agentID)
	if err != nil {
		return nil, err
	}
	have := make(map[uint64]struct{})
	children := make(map[uint64][]uint64)
	for _, t := range topics {
		have[t.ID] = struct{}{}
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t.ID)
		}
	}
	if _, ok := have[root]; !ok {
		return nil, nil
	}
	closure := []uint64{root}
	// Indexed loop, not range: closure grows as children append, and for-range
	// captures the initial length, so the appended levels would never be visited.
	for i := 0; i < len(closure); i++ {
		closure = append(closure, children[closure[i]]...)
	}
	return closure, nil
}
