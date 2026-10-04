// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// L4Index caches one agent domain's content inventory grouped by topic, so a read can enumerate what a
// turn holds.
package index

import (
	"cmp"
	"slices"
	"sync"

	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

type l4Entry struct {
	Seq       uint64
	IDHash    uint64
	Kind      core.ArchiveKind
	CreatedAt int64
	// NodeSeq is the plan step an event names, 0 for a record bound to none.
	NodeSeq uint32
}

type L4Index struct {
	mu      sync.RWMutex
	byTopic map[uint64][]l4Entry // Seq ascending; one entry per (topic, Seq)
}

func NewL4Index() *L4Index {
	return &L4Index{byTopic: make(map[uint64][]l4Entry)}
}

// BuildL4FromEngine scans one agent domain's L4 records into a fresh index, dropping whatever will not
// decode.
func BuildL4FromEngine(engine *core.StorageEngine, agentID uint64) *L4Index {
	idx := NewL4Index()
	for _, arc := range core.CollectAllArchives(engine, agentID) {
		idx.Append(arc.TopicID, arc.Seq, arc.IDHash, arc.Kind, arc.CreatedAt, arc.NodeSeq)
	}
	return idx
}

// Append records one content slot, replacing whatever held that Seq: re-writing a Seq is an in-place
// overwrite on the disk too, so the mirror must not grow.
func (idx *L4Index) Append(topicID, seq, idHash uint64, kind core.ArchiveKind, createdAt int64, nodeSeq uint32) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	entries := idx.byTopic[topicID]
	at, _ := slices.BinarySearchFunc(entries, seq, func(e l4Entry, s uint64) int {
		return cmp.Compare(e.Seq, s)
	})
	e := l4Entry{Seq: seq, IDHash: idHash, Kind: kind, CreatedAt: createdAt, NodeSeq: nodeSeq}
	if at < len(entries) && entries[at].Seq == seq {
		entries[at] = e
		return
	}
	entries = slices.Insert(entries, at, e)
	idx.byTopic[topicID] = entries
}

// MaxNodeSeq returns the highest plan ordinal the topic's records name, 0 when none does.
func (idx *L4Index) MaxNodeSeq(topicID uint64) uint32 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var top uint32
	for _, e := range idx.byTopic[topicID] {
		if e.NodeSeq > top {
			top = e.NodeSeq
		}
	}
	return top
}

// MaxSeq returns the highest Seq the topic holds, 0 when it holds nothing.
func (idx *L4Index) MaxSeq(topicID uint64) uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	entries := idx.byTopic[topicID]
	if len(entries) == 0 {
		return 0
	}
	return entries[len(entries)-1].Seq
}

// IDs returns one kind's record ids in Seq order; nil for a topic with nothing of that kind, which is
// an empty read, not an error.
func (idx *L4Index) IDs(topicID uint64, kind core.ArchiveKind) []uint64 {
	return idx.ids(topicID, &kind)
}

// AllIDs returns every id the topic holds, both kinds, in Seq order.
func (idx *L4Index) AllIDs(topicID uint64) []uint64 {
	return idx.ids(topicID, nil)
}

func (idx *L4Index) ids(topicID uint64, kind *core.ArchiveKind) []uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	entries := idx.byTopic[topicID]
	out := make([]uint64, 0, len(entries))
	for _, e := range entries {
		if kind != nil && e.Kind != *kind {
			continue
		}
		out = append(out, e.IDHash)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ExpiredBefore reports the ids created strictly before cutoff (Unix ms), grouped by topic.
func (idx *L4Index) ExpiredBefore(cutoff int64) map[uint64][]uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	out := make(map[uint64][]uint64)
	for topicID, entries := range idx.byTopic {
		for _, e := range entries {
			if e.CreatedAt < cutoff {
				out[topicID] = append(out[topicID], e.IDHash)
			}
		}
	}
	return out
}

// RemoveIDs drops specific records of one topic — the counterpart of deleting them.
func (idx *L4Index) RemoveIDs(topicID uint64, idHashes []uint64) {
	doomed := make(map[uint64]struct{}, len(idHashes))
	for _, h := range idHashes {
		doomed[h] = struct{}{}
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	entries := idx.byTopic[topicID]
	if len(entries) == 0 {
		return
	}
	kept := slices.DeleteFunc(slices.Clone(entries), func(e l4Entry) bool {
		_, ok := doomed[e.IDHash]
		return ok
	})
	if len(kept) == len(entries) {
		return
	}
	if len(kept) == 0 {
		delete(idx.byTopic, topicID)
		return
	}
	idx.byTopic[topicID] = kept
}

// RemoveTopic drops a whole topic's entries, the counterpart of deleting all the records it owned.
func (idx *L4Index) RemoveTopic(topicID uint64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.byTopic, topicID)
}
