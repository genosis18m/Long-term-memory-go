// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// L2 topic record primitives: listing, creation and the read-modify-write mutations of the keyword
// track and parent link.
package repo

import (
	"cmp"
	"slices"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/index"
)

// TopicListQuery carries ListTopicsL2 inputs.
type TopicListQuery struct {
	MetaIdx *index.L2MetaIndex
	SceneID uint64
	Depth   uint8
}

// ListTopicsL2 lists one scene's topics up to depth.
func ListTopicsL2(q TopicListQuery) []core.TopicSlot {
	depth := q.Depth
	if depth == 0 {
		depth = 1
	} else if depth > MaxDepth {
		depth = MaxDepth
	}
	var out []core.TopicSlot
	for _, meta := range q.MetaIdx.TopicsByScene(q.SceneID) {
		if meta.Depth > depth {
			continue
		}
		out = append(out, meta.ToTopicSlot())
	}
	slices.SortFunc(out, func(a, b core.TopicSlot) int {
		if c := cmp.Compare(a.UserTimestamp, b.UserTimestamp); c != 0 {
			return c
		}
		// A fused parent carries the group's earliest user timestamp, so it ties with the first turn it.
		if c := cmp.Compare(a.Depth, b.Depth); c != 0 {
			return c
		}
		// Whatever still ties needs a fixed order the record scan cannot give.
		return cmp.Compare(a.ID, b.ID)
	})
	return out
}

// RenameTopicL2 writes a caller-chosen name onto one topic.
func RenameTopicL2(engine *core.StorageEngine, agentID uint64, topicID uint64, name string) (*core.TopicSlot, error) {
	topic, err := core.ReadTopicSlot(engine, agentID, topicID)
	if err != nil {
		return nil, err
	}
	if topic.Name == name {
		// Nothing to write: a retried rename would append an equal record and the file charges for it, while
		// the answer the caller gets is the same value either way.
		return topic, nil
	}
	topic.Name = name
	if err := core.WriteTopicSlot(engine, agentID, topic.ID, topic); err != nil {
		return nil, err
	}
	return topic, nil
}

// CreateTurnTopicL2 writes one turn topic under sceneHash with its single keyword track and both
// message timestamps.
func CreateTurnTopicL2(engine *core.StorageEngine, agentID uint64, sceneHash, topicID uint64, keywords []string, userTS, agentTS int64) (*core.TopicSlot, error) {
	topic := core.TopicSlot{
		ID:             topicID,
		SceneID:        sceneHash,
		Depth:          1,
		FusedKeywords:  keywords,
		UserTimestamp:  userTS,
		AgentTimestamp: agentTS,
	}
	stored, err := core.ReadTopicLenient(engine, agentID, topicID)
	switch {
	case err != nil && common.CodeOf(err) != common.ErrNotFound:
		// Nothing stored is this turn's first settle; a record that will not
		// read back is a turn whose place in the tree nobody knows.
		return nil, err
	case err == nil && stored == nil:
		// The address holds a record of another kind; overwriting it comes back
		// as a type it never was — the same answer the sink gives.
		return nil, common.NewError(common.ErrIO,
			common.FormatHash(topicID)+" names a record that is not a topic", nil)
	}
	if stored != nil {
		topic.Name = stored.Name
		topic.Depth = stored.Depth
		topic.ParentID = stored.ParentID
	}
	if err := core.WriteTopicSlot(engine, agentID, topic.ID, &topic); err != nil {
		return nil, err
	}
	return &topic, nil
}

// CreateFusedTopicL2 creates a compressed topic (depth 1) whose Keywords are the fusion of its
// children.
func CreateFusedTopicL2(engine *core.StorageEngine, agentID uint64, sceneID uint64, fusedKeywords []string, userTS, agentTS int64, members []uint64) error {
	topic := core.TopicSlot{
		ID:             core.ComputeFusedTopicID(sceneID, userTS, agentTS, members),
		SceneID:        sceneID,
		Depth:          1,
		UserTimestamp:  userTS,
		AgentTimestamp: agentTS,
		FusedKeywords:  fusedKeywords,
	}
	return core.WriteTopicSlot(engine, agentID, topic.ID, &topic)
}
