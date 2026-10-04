// L0-L5 data models for the MemHop memory database.
package core

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
)

// ProfileSlot is the L0 profile singleton of one agent domain.
type ProfileSlot struct {
	Name         string            `json:"name"`
	Role         string            `json:"role"`
	Personality  string            `json:"personality"`
	EmotionState EmotionScore      `json:"emotion_state"`
	MBTI         MBTIScore         `json:"mbti"`
	Preferences  map[string]string `json:"preferences"`
	AgentType    uint8             `json:"agent_type"`
	UpdatedAtMs  int64             `json:"updated_at_ms"`
}

// Which kind of agent a domain holds.
const (
	AgentTypePrimary uint8 = 0
	AgentTypeSub     uint8 = 1
)

// SceneNode is an L1 hypergraph node linking multiple L2 topics.
type SceneNode struct {
	IDHash     uint64   `json:"id_hash"`
	SceneID    uint64   `json:"scene_id"`
	TopicIDs   []uint64 `json:"topic_ids"`
	Importance float64  `json:"importance"`
	// Valence/Arousal are the emotion signals a distillation computed for this scene on the EmotionScore
	// scale.
	Valence    float64  `json:"valence"`
	Arousal    float64  `json:"arousal"`
	EmotionSet bool     `json:"emotion_set,omitempty"`
	CreatedAt  int64    `json:"created_at"`
	UpdatedAt  int64    `json:"updated_at"`
	EdgeIDs    []uint64 `json:"edge_ids"`
}

// SceneEdge is an L1 hyperedge over a set of member nodes, carrying a weight that decays over time.
type SceneEdge struct {
	IDHash    uint64   `json:"id_hash"`
	NodeIDs   []uint64 `json:"node_ids"`
	Weight    float64  `json:"weight"`
	CreatedAt int64    `json:"created_at"`
	// LastDecayAt: last decay time (ms); 0 = never decayed, first decay starts from CreatedAt.
	LastDecayAt int64 `json:"last_decay_at"`
}

// SceneNodeID derives the stable L1 node ID of a scene: hash("scene-node:"+hex(sceneID)).
func SceneNodeID(sceneID uint64) uint64 {
	return common.HashID("scene-node:" + common.FormatHash(sceneID))
}

// SceneSlot is an L2 scene container.
type SceneSlot struct {
	SceneID   uint64 `json:"scene_id"`
	SceneName string `json:"scene_name"`
	// TurnSeq counts the turns opened here: hash("turn:"+sceneID:TurnSeq) is a turn's topic id, so turn
	// ids never depend on message timestamps.
	TurnSeq uint64 `json:"turn_seq,omitempty"`
	// LastUsedAt is the moment a turn was last opened here (ms).
	LastUsedAt int64  `json:"last_used_at,omitempty"`
	L3ID       uint64 `json:"l3_id"` // project-domain L3 graph this scene is anchored to (N:1)
}

// NewSceneSlot builds a scene record for a caller-supplied scene id and name.
func NewSceneSlot(sceneID uint64, name string) SceneSlot {
	return SceneSlot{
		SceneID:    sceneID,
		SceneName:  name,
		LastUsedAt: time.Now().UnixMilli(),
	}
}

// TopicSlot is one L2 conversation node: a single turn, or a fused group of turns.
type TopicSlot struct {
	ID       uint64  `json:"id"`
	SceneID  uint64  `json:"scene_id"`
	ParentID *uint64 `json:"parent_id,omitempty"`
	Depth    uint8   `json:"depth"`

	// Name is a caller-supplied label nothing here derives into, so rewriting the record goes around it.
	Name string `json:"name,omitempty"`

	FusedKeywords []string `json:"fused_keywords"`

	UserTimestamp  int64 `json:"user_timestamp"`  // turn: user message time; fused: earliest user turn in group
	AgentTimestamp int64 `json:"agent_timestamp"` // turn: agent reply time; fused: latest agent turn in group
}

// CompareTopicOrder orders a scene's topics by the user timestamp they were spoken at, breaking a tie
// on ID so the order is deterministic.
func CompareTopicOrder(a, b TopicSlot) int {
	if a.UserTimestamp != b.UserTimestamp {
		return cmp.Compare(a.UserTimestamp, b.UserTimestamp)
	}
	return cmp.Compare(a.ID, b.ID)
}

// ComputeFusedTopicID derives a consolidation group's parent id from the group's bounds and its member
// set (fused:scene:userTS:agentTS:members).
func ComputeFusedTopicID(sceneID uint64, userTS, agentTS int64, members []uint64) uint64 {
	set := slices.Clone(members)
	slices.Sort(set)
	var b strings.Builder
	fmt.Fprintf(&b, "fused:%d:%d:%d", sceneID, userTS, agentTS)
	for _, m := range set {
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(m, 10))
	}
	return common.HashID(b.String())
}

// ComputeTurnTopicID derives a turn topic's ID from the scene's turn counter rather than its message
// timestamps: the counter form is issuable before a turn's texts exist.
func ComputeTurnTopicID(sceneID, seq uint64) uint64 {
	return common.HashID(fmt.Sprintf("turn:%d:%d", sceneID, seq))
}

// HypergraphSlot holds L3 hypergraph container metadata.
type HypergraphSlot struct {
	IDHash    uint64 `json:"id_hash"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// HypergraphNode is a node within an L3 hypergraph.
type HypergraphNode struct {
	IDHash    uint64   `json:"id_hash"`
	GraphID   uint64   `json:"graph_id"`
	Title     string   `json:"title"`
	NodeType  string   `json:"node_type"`
	Content   string   `json:"content"`
	Keywords  []string `json:"keywords"`
	SourceRef *string  `json:"source_ref,omitempty"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// HypergraphEdge is a hyperedge within an L3 hypergraph: an unordered relation over its member nodes,
// identified by members plus Kind (see repo.CreateEdgeL3).
type HypergraphEdge struct {
	IDHash    uint64        `json:"id_hash"`
	GraphID   uint64        `json:"graph_id"`
	Kind      GraphEdgeKind `json:"kind"`
	NodeIDs   []uint64      `json:"node_ids"`
	CreatedAt int64         `json:"created_at"`
}

// Utterances hold Seq 1 and 2 of their topic.
const (
	SeqUser  uint64 = 1
	SeqAgent uint64 = 2
	// LastUtteranceSeq is the highest slot an utterance may occupy; the first
	// event of a topic is one past it.
	LastUtteranceSeq = SeqAgent
)

// ArchiveSlot stores one piece of a topic's content: a dialogue original (KindUtterance) or an
// operation event (KindEvent).
type ArchiveSlot struct {
	IDHash      uint64      `json:"id_hash"`
	Kind        ArchiveKind `json:"kind"`
	Seq         uint64      `json:"seq"`
	ContentType ContentType `json:"content_type"`
	Role        ArchiveRole `json:"role"`
	TopicID     uint64      `json:"topic_id"`
	EventType   string      `json:"event_type,omitempty"`
	NodeSeq     uint32      `json:"node_seq,omitempty"`
	CreatedAt   int64       `json:"created_at"`
	Content     string      `json:"content"`
}

// HashContent derives the id of one topic's content slot: hash("content:"+topicID+":"+seq).
func HashContent(topicID, seq uint64) uint64 {
	return common.HashID(fmt.Sprintf("content:%d:%d", topicID, seq))
}

// Plan node status.
const (
	StatusInProgress uint8 = 0
	StatusDone       uint8 = 1
	StatusFailed     uint8 = 2
)

// PlanNode is one node of an L5 plan tree, and one node is one record: L5 holds nothing but these.
type PlanNode struct {
	IDHash    uint64 `json:"id_hash"`
	TopicID   uint64 `json:"topic_id"`
	Seq       uint32 `json:"seq"`        // ordinal inside the topic, never 0
	ParentSeq uint32 `json:"parent_seq"` // 0 = root
	Status    uint8  `json:"status"`
	Title     string `json:"title,omitempty"`   // empty = the view falls back to Seq
	Summary   string `json:"summary,omitempty"` // completion abbreviation
	// SummaryFolded marks that Summary is the library's own fold of this step's children, so a later
	// rollup may re-derive it.
	SummaryFolded bool  `json:"summary_folded,omitempty"`
	CreatedAt     int64 `json:"created_at"`            // stamped once, when the node is created
	FinishedAt    int64 `json:"finished_at,omitempty"` // stamped on a terminal status only
	UpdatedAt     int64 `json:"updated_at"`
}

// HashPlanNode derives a plan node id from the owning topic + step ordinal, namespaced under a.
func HashPlanNode(topicID uint64, seq uint32) uint64 {
	return common.HashID(fmt.Sprintf("plan:%d:%d", topicID, seq))
}
