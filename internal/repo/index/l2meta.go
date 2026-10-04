// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

package index

import (
	"slices"
	"sync"

	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// L2Meta is the in-memory cache of one topic record: exactly the fields ToTopicSlot needs to rebuild
// the slot without reading the record.
type L2Meta struct {
	IDHash         uint64
	Depth          uint8
	SceneID        uint64
	ParentID       *uint64
	Name           string
	FusedKeywords  []string
	UserTimestamp  int64
	AgentTimestamp int64
}

// L2MetaIndex caches one row per topic.
type L2MetaIndex struct {
	mu      sync.RWMutex
	entries map[uint64]*L2Meta
	byScene map[uint64][]uint64
}

func newL2MetaIndex() *L2MetaIndex {
	return &L2MetaIndex{
		entries: make(map[uint64]*L2Meta),
		byScene: make(map[uint64][]uint64),
	}
}

// L2MetaFromTopic is the single conversion point from a stored topic record to its cached metadata.
func L2MetaFromTopic(t *core.TopicSlot) *L2Meta {
	return &L2Meta{
		IDHash:         t.ID,
		Depth:          t.Depth,
		SceneID:        t.SceneID,
		ParentID:       t.ParentID,
		Name:           t.Name,
		FusedKeywords:  t.FusedKeywords,
		UserTimestamp:  t.UserTimestamp,
		AgentTimestamp: t.AgentTimestamp,
	}
}

func (idx *L2MetaIndex) Get(idHash uint64) *L2Meta {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.entries[idHash]
}

func (idx *L2MetaIndex) Update(meta *L2Meta) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if old, exists := idx.entries[meta.IDHash]; exists {
		idx.removeFromIndices(old.SceneID, meta.IDHash)
	}
	idx.insertMeta(meta)
}

func (idx *L2MetaIndex) Remove(idHash uint64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	meta, ok := idx.entries[idHash]
	if !ok {
		return
	}
	idx.removeFromIndices(meta.SceneID, idHash)
	delete(idx.entries, idHash)
}

// TopicsByScene is one scene's cached rows, resolved under a single read lock.
func (idx *L2MetaIndex) TopicsByScene(sceneID uint64) []*L2Meta {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	out := make([]*L2Meta, 0, len(idx.byScene[sceneID]))
	for _, id := range idx.byScene[sceneID] {
		out = append(out, idx.entries[id])
	}
	return out
}

// RetargetScene moves every topic of one scene to another in a single write.
func (idx *L2MetaIndex) RetargetScene(fromSceneID, toSceneID uint64) {
	if fromSceneID == toSceneID {
		return
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	ids := idx.byScene[fromSceneID]
	if len(ids) == 0 {
		return
	}
	delete(idx.byScene, fromSceneID)
	idx.byScene[toSceneID] = append(idx.byScene[toSceneID], ids...)
	for _, id := range ids {
		idx.entries[id].SceneID = toSceneID
	}
}

func (idx *L2MetaIndex) insertMeta(meta *L2Meta) {
	idx.entries[meta.IDHash] = meta
	idx.byScene[meta.SceneID] = append(idx.byScene[meta.SceneID], meta.IDHash)
}

func (idx *L2MetaIndex) removeFromIndices(sceneID uint64, idHash uint64) {
	ids, ok := idx.byScene[sceneID]
	if !ok {
		return
	}
	// No clone: this runs under the write lock, and the only way a reader holds a
	// scene's list is through a method that copies it out.
	filtered := slices.DeleteFunc(ids, func(x uint64) bool { return x == idHash })
	if len(filtered) == 0 {
		delete(idx.byScene, sceneID)
	} else {
		idx.byScene[sceneID] = filtered
	}
}

// ToTopicSlot rebuilds the full topic slot from cached metadata.
func (m *L2Meta) ToTopicSlot() core.TopicSlot {
	return core.TopicSlot{
		ID:             m.IDHash,
		SceneID:        m.SceneID,
		ParentID:       m.ParentID,
		Depth:          m.Depth,
		Name:           m.Name,
		FusedKeywords:  m.FusedKeywords,
		UserTimestamp:  m.UserTimestamp,
		AgentTimestamp: m.AgentTimestamp,
	}
}
