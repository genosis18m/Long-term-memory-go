// L4 archive operations of the internal layer: one write and one read over a topic's content, which is
// where a turn's dialogue originals and its operation events both live.

package internal

import (
	"fmt"
	"slices"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/content"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// SearchL4 reads the content records matching every condition of q; the conditions AND together, so an
// empty query returns the domain's whole content set.
func (db *DB) SearchL4(agentID uint64, q L4Query) ([]core.ArchiveSlot, error) {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	if q.NodeSeq != 0 && q.TopicID == nil {
		return nil, common.NewError(common.ErrInvalidQuery,
			"a step filter needs its turn's topic id")
	}
	if q.Kind != nil && !q.Kind.Valid() {
		return nil, common.NewError(common.ErrInvalidQuery, "unknown archive kind")
	}
	if q.Type != nil && !q.Type.Valid() {
		return nil, common.NewError(common.ErrInvalidQuery, "unknown content type")
	}
	// The two time bounds compare against a record's own millisecond stamp, so the same units the write
	// boundary refuses are refused here.
	if err := content.CheckQueryBound("start", q.Start); err != nil {
		return nil, err
	}
	if err := content.CheckQueryBound("end", q.End); err != nil {
		return nil, err
	}
	rq := repo.ArchiveQuery{Keyword: q.Keyword, Start: q.Start, End: q.End, Type: q.Type,
		Kind: q.Kind, Limit: q.Limit, Index: ac.L4}
	if len(q.IDs) > 0 {
		ids, err := common.ParseAll(q.IDs)
		if err != nil {
			return nil, common.NewError(common.ErrInvalidQuery, "parse archive ids", err)
		}
		if slices.Contains(ids, 0) {
			// A named read is the one place a host counts rows instead of listing them, and the reserved key can
			// never name a record: answering one row short would read as "that memory is gone".
			return nil, common.NewError(common.ErrInvalidQuery,
				"archive id 0000000000000000 is reserved and names no record")
		}
		rq.IDs = ids
	}
	if q.TopicID != nil {
		topicHash, err := content.ParseTopicID(*q.TopicID)
		if err != nil {
			return nil, err
		}
		// A scene id and a turn id come out of the same Search result, and transposing them is the one key
		// mistake a host can make without noticing.
		if slot, err := core.ReadSceneSlot(db.engine, agentID, topicHash); err == nil && slot != nil {
			return nil, common.NewError(common.ErrInvalidQuery,
				"topic_id names a scene; the key a turn's own records are addressed by is the NewTopicID Search returned")
		}
		rq.TopicID = &topicHash
		if q.NodeSeq != 0 {
			// The whole branch is expanded here so the data layer only ever
			// matches set membership: it has no view of the tree.
			rq.NodeSeqs = ac.Plans.Subtree(topicHash, q.NodeSeq)
		}
	}
	out, err := repo.QueryArchivesL4(db.engine, agentID, rq)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return []core.ArchiveSlot{}, nil
	}
	return out, nil
}

// AppendArchive writes one piece of the open turn's content and returns the slot it took.
func (db *DB) AppendArchive(agentID uint64, slot core.ArchiveSlot) (uint64, error) {
	ac, err := db.lockTurn(agentID)
	if err != nil {
		return 0, err
	}
	defer ac.Mu.Unlock()
	if slot.Kind == core.KindEvent && slot.NodeSeq != 0 &&
		!ac.Plans.HasSeq(ac.Turn, slot.NodeSeq) {
		return 0, common.NewError(common.ErrInvalidQuery,
			fmt.Sprintf("the event names step %d, which is not on this turn's plan tree",
				slot.NodeSeq))
	}
	return content.Append(ac, agentID, ac.Turn, slot)
}
