// Public type surface of the facade.

package api

import "github.com/genosis18m/Long-term-memory-go/internal"

type (
	// LlmConfig points the engine at its only external service: there is no embedding service and no
	// dimension to declare.
	LlmConfig = internal.LlmConfig
	// MemHopDefaults holds the host-facing business knobs (consolidation thresholds, the idle-domain TTL
	// and the content retention window); engine tuning constants are package-private.
	MemHopDefaults = internal.MemHopDefaults
)

// DefaultMemHopDefaults is the shared default configuration.
var DefaultMemHopDefaults = internal.DefaultMemHopDefaults

// These are the shapes a host fills in.

type (
	// SearchQuery scopes one scene read.
	SearchQuery = internal.SearchQuery
	// TurnEnd is what Update records to close a turn: the stimulus that opened it, the answer that ended
	// it, and the host's own word for which arm it took.
	TurnEnd = internal.TurnEnd
	// L3ImportItem is one knowledge node of an ImportL3 batch: Title names it inside its graph, Domain
	// says which graph, and SourceRef carries a positional reference (file:line, or a URL).
	L3ImportItem = internal.L3ImportItem
	// L3Relation is one import-time hyperedge.
	L3Relation = internal.L3Relation
	// L3ImportMode is ImportL3's policy for a node the graph already holds.
	L3ImportMode = internal.L3ImportMode
	// L3NodeQuery filters the nodes of one graph.
	L3NodeQuery = internal.L3NodeQuery
	// L4Query reads a turn's content.
	L4Query = internal.L4Query
	// ScenePatch is UpdateScene's partial payload: a nil field is left alone, and an empty Name is
	// refused.
	ScenePatch = internal.ScenePatch
	// PlanStatus is a plan step's state: "in_progress" (what a created step is), "done" or "failed".
	PlanStatus = internal.PlanStatus
	// GraphEdgeKind names an L3 relation: related, causal, part_of, sequence, dependency or custom.
	GraphEdgeKind = internal.GraphEdgeKind
	// ContentType is the medium of one L4 record's content: text, image, video,
	// document, audio, code, and 255 ("other") for a medium with no name of its own.
	ContentType = internal.ContentType
	// ArchiveKind says which of a turn's records an L4 slot is (see KindUtterance and
	// KindEvent). It is orthogonal to ContentType, which names the medium of the text.
	ArchiveKind = internal.ArchiveKind
	// ArchiveRole says who spoke an L4 utterance: RoleUser, RoleAgent or RoleSystem on a line a host
	// appended, and RoleDream on the one record the library writes — a fused group's summary.
	ArchiveRole = internal.ArchiveRole
)

// These carry no id the facade has to render, so they are the internal shape.

type (
	// L3ImportResult reports one ImportL3 batch.
	L3ImportResult = internal.L3ImportResult
	// DreamReport is one consolidation pass: what it actually did, stage by stage.
	DreamReport = internal.DreamReport
	// DreamStage is one stage of that pass.
	DreamStage = internal.DreamStage
	// SceneContext is one scene's whole transcript: its topics at depth 1 and 2 in user-timestamp order,
	// each with the dialogue originals it owns.
	SceneContext = internal.SceneContext
	// SceneContextTopic is one topic of that transcript.
	SceneContextTopic = internal.SceneContextTopic
	// SceneMessage is one line of a topic's dialogue, in the Seq order it was written to.
	SceneMessage = internal.SceneMessage
)

// ProfileSlot is the L0 profile as the library hands it back: the fields a host writes plus the ones
// only the library writes; the internal id hash is not part of it.
type ProfileSlot struct {
	Name         string                `json:"name"`
	Role         string                `json:"role"`
	Personality  string                `json:"personality"`
	EmotionState internal.EmotionScore `json:"emotion_state"`
	MBTI         internal.MBTIScore    `json:"mbti"`
	Preferences  map[string]string     `json:"preferences"`
	// AgentType says which kind of agent this domain holds — see AgentTypePrimary.
	// It is stamped when the domain comes to exist, so no host write can move it.
	AgentType   uint8 `json:"agent_type"`
	UpdatedAtMs int64 `json:"updated_at_ms"`
}

// ProfileInput is the profile as a host writes it — the argument to Open, SubAgent and UpdateL0.
type ProfileInput struct {
	Name        string            `json:"name"`
	Role        string            `json:"role"`
	Personality string            `json:"personality"`
	Preferences map[string]string `json:"preferences"`
}

// SceneNodeView is one L1 scene node as a host reads it.
type SceneNodeView struct {
	ID         string   `json:"id"`
	SceneID    string   `json:"scene_id"`
	TopicIDs   []string `json:"topic_ids"`
	EdgeIDs    []string `json:"edge_ids"`
	Importance float64  `json:"importance"`
	Valence    float64  `json:"valence"`
	Arousal    float64  `json:"arousal"`
	EmotionSet bool     `json:"emotion_set"`
	CreatedAt  int64    `json:"created_at"`
	UpdatedAt  int64    `json:"updated_at"`
}

// SceneSlot is one L2 scene container — a host session.
type SceneSlot struct {
	SceneID   string `json:"scene_id"`
	SceneName string `json:"scene_name"`
	L3ID      string `json:"l3_id,omitempty"`
}

// TopicSlot is one L2 conversation node: a single turn closed by Update, or a Dream-fused group of
// turns.
type TopicSlot struct {
	ID             string   `json:"id"`
	SceneID        string   `json:"scene_id"`
	ParentID       *string  `json:"parent_id,omitempty"`
	Depth          uint8    `json:"depth"`
	Name           string   `json:"name,omitempty"`
	FusedKeywords  []string `json:"fused_keywords"`
	UserTimestamp  int64    `json:"user_timestamp"`
	AgentTimestamp int64    `json:"agent_timestamp"`
}

// SearchResult is the read surface of one scene.
type SearchResult struct {
	Profile      ProfileSlot `json:"profile"`
	ProfileBrief string      `json:"profile_brief"`
	Scene        SceneSlot   `json:"scene"`
	Topics       []TopicSlot `json:"topics"`
	NewTopicID   string      `json:"new_topic_id"`
}

// AgentInfo is one agent domain of the file as DB.Agents lists it.
type AgentInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Primary bool   `json:"primary"`
}

// HypergraphSlot is one L3 graph's container metadata.
type HypergraphSlot struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// HypergraphNode is a node within an L3 hypergraph.
type HypergraphNode struct {
	ID        string   `json:"id"`
	GraphID   string   `json:"graph_id"`
	Title     string   `json:"title"`
	NodeType  string   `json:"node_type"`
	Content   string   `json:"content"`
	Keywords  []string `json:"keywords"`
	SourceRef string   `json:"source_ref,omitempty"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// HypergraphEdge is a hyperedge within an L3 hypergraph.
type HypergraphEdge struct {
	ID        string        `json:"id"`
	GraphID   string        `json:"graph_id"`
	Kind      GraphEdgeKind `json:"kind"`
	NodeIDs   []string      `json:"node_ids"`
	CreatedAt int64         `json:"created_at"`
}

// L3Graph is the full view of one L3 hypergraph.
type L3Graph struct {
	Slot  HypergraphSlot   `json:"slot"`
	Nodes []HypergraphNode `json:"nodes"`
	Edges []HypergraphEdge `json:"edges"`
}

// L3Subgraph is a BFS subgraph view.
type L3Subgraph struct {
	Nodes []HypergraphNode `json:"nodes"`
	Edges []HypergraphEdge `json:"edges"`
}

// ArchiveSlot is one record of a topic's L4 content: a dialogue original (KindUtterance) or an
// operation event (KindEvent), as read back.
type ArchiveSlot struct {
	ID          string      `json:"id"`
	Kind        ArchiveKind `json:"kind"`
	Seq         uint64      `json:"seq"`
	ContentType ContentType `json:"content_type"`
	Role        ArchiveRole `json:"role"`
	TopicID     string      `json:"topic_id"`
	EventType   string      `json:"event_type,omitempty"`
	NodeSeq     uint32      `json:"node_seq,omitempty"`
	CreatedAt   int64       `json:"created_at"`
	Content     string      `json:"content"`
}

// ArchiveInput is what AppendArchive hands in: the eight fields a host actually decides about, and no
// others.
type ArchiveInput struct {
	Kind        ArchiveKind `json:"kind"`
	Seq         uint64      `json:"seq"`
	ContentType ContentType `json:"content_type"`
	Role        ArchiveRole `json:"role"`
	EventType   string      `json:"event_type,omitempty"`
	NodeSeq     uint32      `json:"node_seq,omitempty"`
	CreatedAt   int64       `json:"created_at"`
	Content     string      `json:"content"`
}

// PlanNodeView is the external plan-tree node; Status is the string form.
type PlanNodeView struct {
	Seq        uint32         `json:"seq"`
	ParentSeq  uint32         `json:"parent_seq"`
	Title      string         `json:"title"`
	Status     string         `json:"status"`
	Summary    string         `json:"summary"`
	CreatedAt  int64          `json:"created_at"`
	FinishedAt int64          `json:"finished_at"`
	UpdatedAt  int64          `json:"updated_at"`
	Children   []PlanNodeView `json:"children"`
}

// PlanStep is one step restated for PlanNodeUpdate.
type PlanStep struct {
	Seq     uint32     `json:"seq"`
	Title   string     `json:"title"`
	Status  PlanStatus `json:"status"`
	Summary string     `json:"summary"`
}

// PlanTree is the external forest view of one plan: every top-level step is a root, and Done/Total
// count every step of every tree rather than the roots alone.
type PlanTree struct {
	Roots      []PlanNodeView `json:"roots"`
	DoneCount  int            `json:"done_count"`
	TotalCount int            `json:"total_count"`
}
