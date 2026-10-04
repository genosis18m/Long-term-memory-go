// The Session methods split by audience.

package api

import (
	"context"

	"github.com/genosis18m/Long-term-memory-go/internal"
)

// Session binds every call to one agent domain: calls on one domain are serialized by the library's
// domain lock, and the internal session is an unexported field.
type Session struct {
	session *internal.Session
}

// Search reads the scene this domain is working — the host's session: its record plus its depth-1
// topics in turn order, and the topic id this read minted for the turn the host is about to run.
func (s *Session) Search(q SearchQuery) (*SearchResult, error) {
	res, err := s.session.Search(q)
	if err != nil {
		return nil, err
	}
	return fromSearchResult(res), nil
}

// Update closes the turn Search opened for this domain.
func (s *Session) Update(end TurnEnd) (*TopicSlot, error) {
	topic, err := s.session.Update(end)
	if err != nil {
		return nil, err
	}
	out := fromTopicSlot(*topic)
	return &out, nil
}

// GetL0 returns the domain's profile.
func (s *Session) GetL0() (*ProfileSlot, error) {
	slot, err := s.session.GetL0()
	if err != nil {
		return nil, err
	}
	out := fromProfileSlot(*slot)
	return &out, nil
}

// UpdateL0 writes the host-owned profile fields (Name / Role / Personality / Preferences).
func (s *Session) UpdateL0(profile ProfileInput) error {
	coreSlot := toCoreProfileSlot(profile)
	return s.session.UpdateL0(&coreSlot)
}

// ListL1 returns the domain's L1 scene nodes in an order stable across calls.
func (s *Session) ListL1() ([]SceneNodeView, error) {
	nodes, err := s.session.ListL1()
	if err != nil {
		return nil, err
	}
	return mapSlice(nodes, fromSceneNode), nil
}

// ListScenes returns the domain's scenes; a non-empty l3ID keeps only the scenes anchored to that L3
// project domain.
func (s *Session) ListScenes(l3ID string) ([]SceneSlot, error) {
	scenes, err := s.session.ListScenes(l3ID)
	if err != nil {
		return nil, err
	}
	return mapSlice(scenes, fromSceneSlot), nil
}

// UpdateScene patches one scene's host-facing metadata (title, L3 anchor) and returns the scene as
// stored afterwards, so a host confirms an anchor without listing the domain.
func (s *Session) UpdateScene(sceneID string, patch ScenePatch) (SceneSlot, error) {
	slot, err := s.session.UpdateScene(sceneID, patch)
	if err != nil {
		return SceneSlot{}, err
	}
	return fromSceneSlot(slot), nil
}

// RenameTopic gives one topic the name the host chose and returns the topic as stored afterwards.
func (s *Session) RenameTopic(topicID, name string) (TopicSlot, error) {
	slot, err := s.session.RenameTopic(topicID, name)
	if err != nil {
		return TopicSlot{}, err
	}
	return fromTopicSlot(slot), nil
}

// GetL3 returns one L3 graph.
func (s *Session) GetL3(id string) (*L3Graph, error) {
	g, err := s.session.GetL3(id)
	if err != nil {
		return nil, err
	}
	return fromL3Graph(g), nil
}

// ListL3 returns all hypergraph slots, sorted by graph id.
func (s *Session) ListL3() ([]HypergraphSlot, error) {
	graphs, err := s.session.ListL3()
	if err != nil {
		return nil, err
	}
	return mapSlice(graphs, fromHypergraphSlot), nil
}

// UpdateL3 renames a graph and returns it.
func (s *Session) UpdateL3(id string, name string) (*L3Graph, error) {
	g, err := s.session.UpdateL3(id, name)
	if err != nil {
		return nil, err
	}
	return fromL3Graph(g), nil
}

// QueryL3Nodes returns the nodes one L3NodeQuery keeps, sorted by node id; Limit keeps the first N of
// that order, so a capped query is the same subset every time.
func (s *Session) QueryL3Nodes(q L3NodeQuery) ([]HypergraphNode, error) {
	nodes, err := s.session.QueryL3Nodes(q)
	if err != nil {
		return nil, err
	}
	return mapSlice(nodes, fromHypergraphNode), nil
}

// QueryL3Subgraph returns a BFS subgraph, its nodes and edges sorted by id.
func (s *Session) QueryL3Subgraph(graphID, startNodeID string, maxDepth int, edgeKinds []GraphEdgeKind) (*L3Subgraph, error) {
	sub, err := s.session.QueryL3Subgraph(graphID, startNodeID, maxDepth, edgeKinds)
	if err != nil {
		return nil, err
	}
	return fromL3Subgraph(sub), nil
}

// SearchL4 returns the content records an L4Query keeps.
func (s *Session) SearchL4(q L4Query) ([]ArchiveSlot, error) {
	archives, err := s.session.SearchL4(q)
	if err != nil {
		return nil, err
	}
	return mapSlice(archives, fromArchiveSlot), nil
}

// AppendArchive writes one piece of the open turn's content and is the only way content enters a
// topic.
func (s *Session) AppendArchive(in ArchiveInput) (uint64, error) {
	return s.session.AppendArchive(toCoreAppendSlot(in))
}

// PlanNodeAdd adds one step to the open turn's plan tree and returns its ordinal.
func (s *Session) PlanNodeAdd(parentSeq uint32, title string) (uint32, error) {
	return s.session.PlanNodeAdd(parentSeq, title)
}

// PlanNodeUpdate restates one step of a turn's plan tree.
func (s *Session) PlanNodeUpdate(step PlanStep) error {
	return s.session.PlanNodeUpdate(toInternalPlanStep(step))
}

// PlanState returns the plan tree of the turn Search opened for this domain.
func (s *Session) PlanState() (*PlanTree, error) {
	t, err := s.session.PlanState()
	if err != nil {
		return nil, err
	}
	out := fromPlanTree(t)
	return &out, nil
}

// AgentID is the id of the domain this handle is bound to, as 16 hex characters, and inside this file
// it names exactly one domain.
func (s *Session) AgentID() string { return formatID(s.session.AgentID()) }

// These methods need no DTO mapping.

// Dream runs the consolidation pass.
func (s *Session) Dream(ctx context.Context, sceneID string) (*DreamReport, error) {
	return s.session.Dream(ctx, sceneID)
}

// SceneContext reads a scene's whole transcript without opening a turn: unlike Search it writes
// nothing.
func (s *Session) SceneContext(sceneID string) (*SceneContext, error) {
	return s.session.SceneContext(sceneID)
}

// MergeScenes folds scenes together: every topic of each secondary scene is retargeted to the primary
// scene, then the secondary scene records are deleted.
func (s *Session) MergeScenes(primaryID string, secondaryIDs []string) error {
	return s.session.MergeScenes(primaryID, secondaryIDs)
}

// DeleteScene removes a scene for good: its record, every topic at any depth, all of their L4 content
// (originals and events alike), the plan trees those turns opened and the L1 scene node.
func (s *Session) DeleteScene(sceneID string) error {
	return s.session.DeleteScene(sceneID)
}

// DeleteTopic removes one topic and its whole subtree (children at any depth) with all of their L4
// content and the plan trees those turns opened — the memory-correction counterpart of Update.
func (s *Session) DeleteTopic(topicID string) error {
	return s.session.DeleteTopic(topicID)
}

// ImportL3 batch-imports knowledge nodes into one graph per Domain: a domain name the graph already
// has extends it, a new one creates it.
func (s *Session) ImportL3(items []L3ImportItem, mode L3ImportMode) (*L3ImportResult, error) {
	return s.session.ImportL3(items, mode)
}

// DeleteL3 removes a whole graph: the slot plus all of its nodes and hyperedges.
func (s *Session) DeleteL3(id string) error {
	return s.session.DeleteL3(id)
}
