// Business DTOs of the storage-layer model package: pure request and response shapes, no methods and
// no logic.

package core

// SearchQuery is one scene-scoped read.
type SearchQuery struct {
	SceneID  string `json:"scene_id,omitempty"`
	L3ID     string `json:"l3_id,omitempty"`
	NewScene bool   `json:"new_scene,omitempty"`
}

// SearchResult carries the L0 profile plus the read surface of one scene: the scene record, its
// depth-1 topics, and NewTopicID — the topic this read opened.
type SearchResult struct {
	Profile      ProfileSlot `json:"profile"`
	ProfileBrief string      `json:"profile_brief"`
	Scene        SceneSlot   `json:"scene"`
	Topics       []TopicSlot `json:"topics"`
	NewTopicID   uint64      `json:"new_topic_id"`
}

// TurnEnd is what one turn leaves behind when its host closes it.
type TurnEnd struct {
	Input     string `json:"input"`
	Output    string `json:"output"`
	Outcome   string `json:"outcome,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

// SceneMessage is one L4 utterance inside a scene context topic.
type SceneMessage struct {
	Role      ArchiveRole `json:"role"`
	Type      ContentType `json:"type"`
	Content   string      `json:"content"`
	Seq       uint64      `json:"seq"`
	CreatedAt int64       `json:"created_at"`
}

// SceneContextTopic is one topic of a scene context with its L4 messages and its child count.
type SceneContextTopic struct {
	TopicID        string         `json:"topic_id"`
	Depth          int            `json:"depth"`
	Name           string         `json:"name,omitempty"`
	Keywords       []string       `json:"keywords"`
	Messages       []SceneMessage `json:"messages,omitempty"`
	ChildCount     int            `json:"child_count"`
	UserTimestamp  int64          `json:"user_timestamp"`
	AgentTimestamp int64          `json:"agent_timestamp"`
}

// SceneContext is a scene's whole transcript, flattened to depth 2 on purpose: a Dream-fused group
// keeps its originals on the child topics it sunk, and this is the only read that brings them back.
type SceneContext struct {
	SceneName string              `json:"scene_name"`
	Topics    []SceneContextTopic `json:"topics"`
}

type L3Graph struct {
	Slot  HypergraphSlot
	Nodes []HypergraphNode
	Edges []HypergraphEdge
}

// L3ImportItem is one knowledge node of a batch import; SourceRef carries a positional reference.
type L3ImportItem struct {
	Title     string       `json:"title"`
	Domain    string       `json:"domain"`
	NodeType  string       `json:"node_type"`
	Content   string       `json:"content"`
	Keywords  []string     `json:"keywords"`
	SourceRef string       `json:"source_ref,omitempty"`
	Related   []L3Relation `json:"related,omitempty"`
}

// L3Relation is one import-time hyperedge: the member nodes of a single relation, named by title
// inside the same graph.
type L3Relation struct {
	Titles []string      `json:"titles"`
	Kind   GraphEdgeKind `json:"kind,omitempty"`
}

// L3ImportResult reports one import batch.
type L3ImportResult struct {
	GraphIDs     []string `json:"graph_ids"`
	CreatedIDs   []string `json:"created_ids"`
	UpdatedIDs   []string `json:"updated_ids"`
	SkippedCount int      `json:"skipped_count"`
	EdgesCreated int      `json:"edges_created,omitempty"`
	Errors       []string `json:"errors"`
}

// L3NodeQuery is a node query over one graph: GraphID is required and every other condition that is
// set filters, so IDs/Keyword/NodeType AND together.
type L3NodeQuery struct {
	GraphID  string   `json:"graph_id"`
	IDs      []string `json:"ids,omitempty"`
	Keyword  string   `json:"keyword,omitempty"`
	NodeType string   `json:"node_type,omitempty"`
	Limit    int      `json:"limit,omitempty"` // <=0 means unlimited
}

type L3Subgraph struct {
	Nodes []HypergraphNode
	Edges []HypergraphEdge
}

// L4Query archive query: every field is optional and the set conditions AND together, so an empty
// query selects the domain's whole content set.
type L4Query struct {
	Keyword string   `json:"keyword,omitempty"`  // case-insensitive substring of Content
	Start   int64    `json:"start,omitempty"`    // created at or after (ms); 0 leaves the bound unset, a wrong scale is refused
	End     int64    `json:"end,omitempty"`      // created at or before (ms); same ruler as a write's CreatedAt
	IDs     []string `json:"ids,omitempty"`      // only these archive ids (the reserved key is refused)
	TopicID *string  `json:"topic_id,omitempty"` // only archives of this turn; a scene id
	// // there is refused, an unknown id answered empty.
	Type    *ContentType `json:"type,omitempty"`     // only archives of this content type
	Kind    *ArchiveKind `json:"kind,omitempty"`     // utterance or event; unset selects both
	NodeSeq uint32       `json:"node_seq,omitempty"` // this step and every step under it; needs TopicID
	Limit   int          `json:"limit,omitempty"`    // keep the tail of the read's order; <=0 means every match
}

// ScenePatch is the partial-update payload of UpdateScene; nil fields are left unchanged.
type ScenePatch struct {
	Name  *string `json:"name,omitempty"`
	L3ID  *string `json:"l3_id,omitempty"`
	Force bool    `json:"force,omitempty"`
}

// DreamStage is one pipeline phase's outcome inside a DreamReport.
type DreamStage struct {
	Name       string `json:"name"`   // l4_prune/l5_prune/l2_compress/index_rebuild/l1_nodes/l1_hyperedges/l1_rebuild/l1_decay/l0_distill
	Status     string `json:"status"` // ok | skipped | cancelled | error
	DurationMs int64  `json:"duration_ms"`
}

// DreamReport is one consolidation pass's structured result; counts describe what this pass actually
// did.
type DreamReport struct {
	L4RecordsPruned    int          `json:"l4_records_pruned"`    // L4 records past the retention window
	L5NodesPruned      int          `json:"l5_nodes_pruned"`      // plan nodes the sweep took with them
	ConsolidatedScenes int          `json:"consolidated_scenes"`  // scenes with >=1 applied merge group
	L2TopicsCompressed int          `json:"l2_topics_compressed"` // topics sunk into groups, not group count
	L1NodesAdded       int          `json:"l1_nodes_added"`       // scene nodes created or updated by the sync
	L1EdgesAdded       int          `json:"l1_edges_added"`       // hyperedges created or strengthened
	L1NodesRemoved     int          `json:"l1_nodes_removed"`     // stale rebuild + decay removals
	L1EdgesRemoved     int          `json:"l1_edges_removed"`     // edges taken by stale rebuild and decay
	L0Updated          bool         `json:"l0_updated"`           // emotion/MBTI distillation ran and wrote back
	Stages             []DreamStage `json:"stages,omitempty"`
}

// L3ImportMode selects the conflict policy of ImportL3.
type L3ImportMode string

const (
	L3ImportSkip      L3ImportMode = "skip"
	L3ImportMerge     L3ImportMode = "merge"
	L3ImportOverwrite L3ImportMode = "overwrite"
)

// Valid reports whether m is one of the three defined modes.
func (m L3ImportMode) Valid() bool {
	switch m {
	case L3ImportSkip, L3ImportMerge, L3ImportOverwrite:
		return true
	}
	return false
}
