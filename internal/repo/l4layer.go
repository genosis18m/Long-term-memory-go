// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

package repo

import (
	"cmp"
	"slices"
	"strings"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/index"
)

// L4 content operations.

// AppendArchiveL4 writes one content slot.
func AppendArchiveL4(engine *core.StorageEngine, agentID uint64, idx *index.L4Index, arc *core.ArchiveSlot) error {
	arc.IDHash = core.HashContent(arc.TopicID, arc.Seq)
	if err := core.WriteArchiveSlot(engine, agentID, arc.IDHash, arc); err != nil {
		return err
	}
	idx.Append(arc.TopicID, arc.Seq, arc.IDHash, arc.Kind, arc.CreatedAt, arc.NodeSeq)
	return nil
}

// DeleteTopicArchives tombstones every content record the index credits the given topics with, both
// kinds, then drops those topics from the index.
func DeleteTopicArchives(engine *core.StorageEngine, agentID uint64, idx *index.L4Index, topics []uint64) error {
	var doomed []uint64
	for _, topicID := range topics {
		doomed = append(doomed, idx.AllIDs(topicID)...)
	}
	if _, err := engine.DeleteRecordBatch(agentID, common.DedupSorted(doomed)); err != nil {
		return common.NewError(common.ErrIO, "delete l4 content", err)
	}
	for _, topicID := range topics {
		idx.RemoveTopic(topicID)
	}
	return nil
}

// DropExpiredArchives tombstones every content record the index reports as created before cutoff and
// mirrors the removal, returning how many went away.
func DropExpiredArchives(engine *core.StorageEngine, agentID uint64, idx *index.L4Index, cutoff int64) (int, error) {
	expired := idx.ExpiredBefore(cutoff)
	var all []uint64
	for _, ids := range expired {
		all = append(all, ids...)
	}
	n, err := engine.DeleteRecordBatch(agentID, all)
	if err != nil {
		return 0, common.NewError(common.ErrIO, "drop expired content", err)
	}
	for topicID, ids := range expired {
		idx.RemoveIDs(topicID, ids)
	}
	return n, nil
}

// ArchiveQuery is the L4 read filter: every field is optional and the set conditions AND together, so
// an empty query selects the domain's whole content set — utterances AND events alike.
type ArchiveQuery struct {
	IDs      []uint64
	TopicID  *uint64
	Type     *core.ContentType
	Kind     *core.ArchiveKind
	NodeSeqs []uint32
	Keyword  string
	Start    int64
	End      int64
	Limit    int
	Index    *index.L4Index
}

// QueryArchivesL4 returns the content records matching every set condition.
func QueryArchivesL4(engine *core.StorageEngine, agentID uint64, q ArchiveQuery) ([]core.ArchiveSlot, error) {
	q.Keyword = strings.ToLower(q.Keyword)
	var out []core.ArchiveSlot
	var err error
	switch {
	case q.TopicID != nil && q.Index != nil:
		out, err = ReadArchivesByIDs(engine, agentID, q.Index.AllIDs(*q.TopicID))
	case len(q.IDs) > 0 && q.TopicID == nil && q.Type == nil && q.Kind == nil &&
		len(q.NodeSeqs) == 0 && q.Keyword == "" && q.Start == 0 && q.End == 0:
		out, err = listByID(engine, agentID, q.IDs, func(id uint64) (*core.ArchiveSlot, error) {
			return core.ReadArchiveSlot(engine, agentID, id)
		})
	default:
		// A read, not a cache rebuild: a record the index names but the payload
		// will not decode must not come back as a shorter transcript.
		out, err = core.CollectAllStrict[core.ArchiveSlot](engine, agentID, core.RecL4Archive)
	}
	if err != nil {
		return nil, err
	}
	filtered := out[:0]
	for _, arc := range out {
		if matchesArchiveQuery(arc, q) {
			filtered = append(filtered, arc)
		}
	}
	slices.SortFunc(filtered, compareArchives(q.TopicID != nil))
	return newest(filtered, q.Limit), nil
}

// compareArchives picks the read order, and the two cases are not the same order.
func compareArchives(oneTopic bool) func(a, b core.ArchiveSlot) int {
	if oneTopic {
		return func(a, b core.ArchiveSlot) int { return cmp.Compare(a.Seq, b.Seq) }
	}
	return func(a, b core.ArchiveSlot) int {
		if c := cmp.Compare(a.CreatedAt, b.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.IDHash, b.IDHash)
	}
}

// newest keeps the last limit entries of an ascending result.
func newest(out []core.ArchiveSlot, limit int) []core.ArchiveSlot {
	if limit <= 0 || len(out) <= limit {
		return out
	}
	return out[len(out)-limit:]
}

// ReadArchivesByIDs loads the records one content read selected.
func ReadArchivesByIDs(engine *core.StorageEngine, agentID uint64, ids []uint64) ([]core.ArchiveSlot, error) {
	out := make([]core.ArchiveSlot, 0, len(ids))
	for _, idHash := range ids {
		arc, err := core.ReadArchiveSlot(engine, agentID, idHash)
		if err != nil {
			if common.CodeOf(err) == common.ErrNotFound {
				return nil, common.NewError(common.ErrIO, "content index names a missing record", err)
			}
			return nil, err
		}
		out = append(out, *arc)
	}
	return out, nil
}

func matchesArchiveQuery(arc core.ArchiveSlot, q ArchiveQuery) bool {
	if len(q.IDs) > 0 && !slices.Contains(q.IDs, arc.IDHash) {
		return false
	}
	if q.TopicID != nil && arc.TopicID != *q.TopicID {
		return false
	}
	if q.Kind != nil && arc.Kind != *q.Kind {
		return false
	}
	if len(q.NodeSeqs) > 0 && !slices.Contains(q.NodeSeqs, arc.NodeSeq) {
		return false
	}
	if q.Type != nil && arc.ContentType != *q.Type {
		return false
	}
	if q.Keyword != "" && !strings.Contains(strings.ToLower(arc.Content), q.Keyword) {
		return false
	}
	if q.Start > 0 && arc.CreatedAt < q.Start {
		return false
	}
	if q.End > 0 && arc.CreatedAt > q.End {
		return false
	}
	return true
}
