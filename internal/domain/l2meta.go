// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

package domain

import (
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/index"
)

// SyncL2Meta refreshes one topic entry of the agent's L2MetaIndex; call it with the slot right after
// the engine writes it.
func (c *Context) SyncL2Meta(topic *core.TopicSlot) {
	c.L2Meta.Update(index.L2MetaFromTopic(topic))
}

// RemoveTopicsFromIndices drops the given topics from the L2Meta cache and from the plan trees they
// owned: a cached tree outliving its topic could keep a dead plan alive.
func (c *Context) RemoveTopicsFromIndices(ids []uint64) {
	for _, id := range ids {
		c.L2Meta.Remove(id)
		c.Plans.RemoveTopic(id)
	}
}

// RetargetL2Meta moves every topic of the merged-away scenes to the primary scene in the L2Meta cache;
// call it once the merge has been applied to the records.
func (c *Context) RetargetL2Meta(primaryHash uint64, removed map[uint64]struct{}) {
	for sid := range removed {
		c.L2Meta.RetargetScene(sid, primaryHash)
	}
}

// ForgetScene drops the domain's memory of its current scene, and with it the turn opened on it: the
// next read then restores from the records instead of resuming a scene that no longer exists.
func (c *Context) ForgetScene(sceneID uint64) {
	if c.Scene == sceneID {
		c.Scene, c.Turn = 0, 0
	}
}

// MoveScene retargets the domain's current scene when a merge swallows it.
func (c *Context) MoveScene(from, to uint64) {
	if c.Scene == from {
		c.Scene, c.Turn = to, 0
	}
}

// ForgetTurn clears the open turn when the topic that held it is deleted, so a later close is refused
// rather than written onto a turn that is gone.
func (c *Context) ForgetTurn(topicID uint64) {
	if c.Turn == topicID {
		c.Turn = 0
	}
}
