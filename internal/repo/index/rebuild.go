// L2 topic metadata rebuild: one engine scan over an agent domain's topic records builds the whole
// cache in one pass.
package index

import "github.com/genosis18m/Long-term-memory-go/internal/repo/core"

// BuildL2MetaFromEngine fills an L2MetaIndex from one agent domain's topic records in a single scan.
func BuildL2MetaFromEngine(engine *core.StorageEngine, agentID uint64) *L2MetaIndex {
	l2Meta := newL2MetaIndex()
	for topic := range core.IterAll[core.TopicSlot](engine, agentID, core.RecL2Topic) {
		l2Meta.insertMeta(L2MetaFromTopic(&topic))
	}
	return l2Meta
}
