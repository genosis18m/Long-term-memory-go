// L2 scene big methods of the composition root: list / metadata patch / topic naming / merge / delete
// and the deep scene-context read.

package internal

import (
	"fmt"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/content"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
	"github.com/genosis18m/Long-term-memory-go/internal/scene"
)

// ListScenes returns the domain's scenes, optionally filtered by their L3 project-domain anchor: an
// empty l3ID lists every scene.
func (db *DB) ListScenes(agentID uint64, l3ID string) ([]core.SceneSlot, error) {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	all, err := repo.CollectAllScenesL2(db.engine, agentID)
	if err != nil {
		return nil, err
	}
	if l3ID == "" {
		if all == nil {
			return []core.SceneSlot{}, nil
		}
		return all, nil
	}
	l3Hash, err := common.ParseID(l3ID)
	if err != nil {
		return nil, common.NewError(common.ErrInvalidQuery, "parse l3 id", err)
	}
	out := []core.SceneSlot{}
	for _, s := range all {
		if s.L3ID == l3Hash {
			out = append(out, s)
		}
	}
	return out, nil
}

// UpdateScene corrects a scene's host-facing metadata in one write and returns the scene as stored
// afterwards.
func (db *DB) UpdateScene(agentID uint64, sceneID string, patch ScenePatch) (core.SceneSlot, error) {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return core.SceneSlot{}, err
	}
	defer ac.Mu.Unlock()
	if patch.Name != nil && *patch.Name == "" {
		return core.SceneSlot{}, common.NewError(common.ErrInvalidQuery, "scene name is required")
	}
	sceneHash, err := common.ParseID(sceneID)
	if err != nil {
		return core.SceneSlot{}, common.NewError(common.ErrInvalidQuery, "parse scene id", err)
	}
	var l3Hash uint64
	if patch.L3ID != nil && *patch.L3ID != "" {
		anchorID, err := parseID("l3", *patch.L3ID)
		if err != nil {
			return core.SceneSlot{}, err
		}
		g, err := repo.ReadSharedGraphL3(db.engine, anchorID)
		if err != nil {
			return core.SceneSlot{}, err
		}
		l3Hash = g.IDHash
	}
	slot, err := core.ReadSceneSlot(db.engine, agentID, sceneHash)
	if err != nil {
		return core.SceneSlot{}, err
	}
	stored := *slot
	if patch.Name != nil {
		slot.SceneName = *patch.Name
	}
	if patch.L3ID != nil {
		// Only replacing one anchor with another is destructive enough to need Force: the old value is lost.
		if l3Hash != 0 && slot.L3ID != 0 && slot.L3ID != l3Hash && !patch.Force {
			return core.SceneSlot{}, common.NewError(common.ErrInvalidQuery,
				fmt.Sprintf("scene %s is anchored to %s; pass Force to re-anchor", sceneID, common.FormatHash(slot.L3ID)))
		}
		slot.L3ID = l3Hash
	}
	if *slot == stored {
		// The patch carried nothing this record does not already hold — an empty patch is exactly what a host
		// sends to confirm a scene and its anchor without listing the domain.
		return stored, nil
	}
	if err := core.WriteSceneSlot(db.engine, agentID, sceneHash, slot); err != nil {
		return core.SceneSlot{}, err
	}
	return *slot, nil
}

// RenameTopic gives one topic the name its host chose and returns the topic as stored afterwards.
func (db *DB) RenameTopic(agentID uint64, topicID, name string) (core.TopicSlot, error) {
	if name == "" {
		return core.TopicSlot{}, common.NewError(common.ErrInvalidQuery, "topic name is required")
	}
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return core.TopicSlot{}, err
	}
	defer ac.Mu.Unlock()
	parsed, err := content.ParseTopicID(topicID)
	if err != nil {
		return core.TopicSlot{}, err
	}
	slot, err := repo.RenameTopicL2(db.engine, agentID, parsed, name)
	if err != nil {
		return core.TopicSlot{}, err
	}
	// The scene read serves this topic out of the cache, so the record write has to be mirrored here or
	// the new name stays invisible until the next consolidation rebuilds the index.
	ac.SyncL2Meta(slot)
	return *slot, nil
}

// MergeScenes rewrites all topics of secondary scenes to the primary scene and deletes the secondary
// records.
func (db *DB) MergeScenes(agentID uint64, primaryID string, secondaryIDs []string) error {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return err
	}
	defer ac.Mu.Unlock()
	primaryHash, err := common.ParseID(primaryID)
	if err != nil {
		return common.NewError(common.ErrInvalidQuery, "parse primary scene id", err)
	}
	hashes, err := common.ParseAll(secondaryIDs)
	if err != nil {
		return common.NewError(common.ErrInvalidQuery, "parse secondary scene ids", err)
	}
	if len(hashes) == 0 {
		return common.NewError(common.ErrInvalidQuery, "secondary scene ids are required")
	}
	if _, dup := common.ToSet(hashes)[primaryHash]; dup {
		return common.NewError(common.ErrInvalidQuery, "primary scene id must not be a secondary", nil)
	}
	if len(common.ToSet(hashes)) != len(hashes) {
		// A merge is destructive and this list is its input.
		return common.NewError(common.ErrInvalidQuery, "a secondary scene id is listed twice", nil)
	}
	// A merge destroys records, so every id it names must still be a scene.
	if err := db.requireScenes(agentID, append([]uint64{primaryHash}, hashes...)...); err != nil {
		return err
	}
	// An anchor is a membership rather than a label: the merged conversation still belongs to whichever
	// project domain one of these scenes was anchored to.
	anchor, err := mergedAnchor(db.engine, agentID, primaryHash, hashes)
	if err != nil {
		return err
	}
	// A merged scene's L1 node goes with it: the merge retargets its topics to the primary, and nothing
	// names the secondary's node again.
	for _, secondary := range hashes {
		if err := repo.DeleteSceneNodeL1(db.engine, agentID, secondary); err != nil {
			return err
		}
	}
	if err := repo.MergeScenesL2(db.engine, agentID, primaryHash, hashes); err != nil {
		return err
	}
	if anchor != 0 {
		slot, err := core.ReadSceneSlot(db.engine, agentID, primaryHash)
		if err != nil {
			return err
		}
		slot.L3ID = anchor
		if err := core.WriteSceneSlot(db.engine, agentID, primaryHash, slot); err != nil {
			return err
		}
	}
	// Mirror the scene retarget in the L2MetaIndex so cached topics match the merged records (storage
	// write already done), and move the domain's own memory of which scene it is on.
	ac.RetargetL2Meta(primaryHash, common.ToSet(hashes))
	for _, secondary := range hashes {
		ac.MoveScene(secondary, primaryHash)
	}
	return nil
}

// mergedAnchor decides which L3 domain the surviving scene belongs to once the others are folded into
// it.
func mergedAnchor(engine *core.StorageEngine, agentID, primary uint64, secondaries []uint64) (uint64, error) {
	survivor, err := core.ReadSceneSlot(engine, agentID, primary)
	if err != nil {
		return 0, err
	}
	if survivor.L3ID != 0 {
		return 0, nil
	}
	var distinct []uint64
	for _, id := range secondaries {
		secondary, err := core.ReadSceneSlot(engine, agentID, id)
		if err != nil {
			return 0, err
		}
		if secondary.L3ID == 0 {
			continue
		}
		seen := false
		for _, g := range distinct {
			if g == secondary.L3ID {
				seen = true
			}
		}
		if !seen {
			distinct = append(distinct, secondary.L3ID)
		}
	}
	switch len(distinct) {
	case 0:
		return 0, nil
	case 1:
		return distinct[0], nil
	default:
		return 0, common.NewError(common.ErrInvalidQuery,
			fmt.Sprintf("the scenes being merged are anchored to %d different L3 domains; anchor the survivor first, or merge one domain at a time", len(distinct)))
	}
}

// requireScenes resolves every named id against the scene records and reports the first that is not
// one.
func (db *DB) requireScenes(agentID uint64, ids ...uint64) error {
	scenes, err := repo.ListScenesL2(db.engine, agentID, ids)
	if err != nil {
		return err
	}
	have := make(map[uint64]struct{}, len(scenes))
	for _, s := range scenes {
		have[s.SceneID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := have[id]; !ok {
			return common.NewError(common.ErrNotFound, "scene not found: "+common.FormatHash(id), nil)
		}
	}
	return nil
}

// SceneContext returns one scene's transcript — topics with depth <= 2 in user timestamp order plus
// their L4 messages — and writes nothing.
func (db *DB) SceneContext(agentID uint64, sceneID string) (*SceneContext, error) {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	sceneHash, hasScene, err := db.readScene(ac, agentID, sceneID)
	if err != nil {
		return nil, err
	}
	if !hasScene {
		return &SceneContext{Topics: []SceneContextTopic{}}, nil
	}
	scenes, err := repo.ListScenesL2(db.engine, agentID, []uint64{sceneHash})
	if err != nil {
		return nil, err
	}
	if len(scenes) == 0 {
		return nil, common.NewError(common.ErrNotFound, "scene not found", nil)
	}
	topics := repo.ListTopicsL2(repo.TopicListQuery{
		MetaIdx: ac.L2Meta,
		SceneID: sceneHash,
		Depth:   2,
	})
	children := make(map[uint64]int)
	for _, t := range topics {
		if t.ParentID != nil {
			children[*t.ParentID]++
		}
	}
	out := &SceneContext{SceneName: scenes[0].SceneName, Topics: []SceneContextTopic{}}
	for _, t := range topics {
		utterances, err := content.Read(agentID, ac, t.ID, core.KindUtterance)
		if err != nil {
			return nil, err
		}
		out.Topics = append(out.Topics, scene.ContextTopic(t, children, utterances))
	}
	return out, nil
}

// DeleteTopic removes a topic and its whole subtree (children at any depth), the L4 archives and plan.
func (db *DB) DeleteTopic(agentID uint64, topicID string) error {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return err
	}
	defer ac.Mu.Unlock()
	parsedID, err := content.ParseTopicID(topicID)
	if err != nil {
		return err
	}
	topics, err := repo.TopicClosureL2(db.engine, agentID, parsedID)
	if err != nil {
		return err
	}
	if len(topics) == 0 {
		return common.NewError(common.ErrNotFound, "topic not found")
	}
	if err := scene.DeleteCascade(ac, agentID, nil, topics); err != nil {
		return err
	}
	// The whole closure goes, so the open turn may be anywhere inside it: a fused
	// group deleted from the scene's surface takes the turn it swallowed.
	for _, id := range topics {
		ac.ForgetTurn(id)
	}
	return nil
}

// DeleteScene removes a scene.
func (db *DB) DeleteScene(agentID uint64, sceneID string) error {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return err
	}
	defer ac.Mu.Unlock()
	sceneHash, err := common.ParseID(sceneID)
	if err != nil {
		return common.NewError(common.ErrInvalidQuery, "parse scene id", err)
	}
	if _, err := core.ReadSceneSlot(db.engine, agentID, sceneHash); err != nil {
		return err
	}
	// The scene's topics are enumerated once, here, and the cascade deletes from that
	// list: a domain that will not read back is refused before any record goes.
	topics, err := repo.TopicIDsBySceneL2(db.engine, agentID, sceneHash)
	if err != nil {
		return err
	}
	if err := scene.DeleteCascade(ac, agentID, []uint64{sceneHash}, topics); err != nil {
		return err
	}
	// Drop the L1 scene node right away (its ID is derivable without an index).
	if err := repo.DeleteSceneNodeL1(db.engine, agentID, sceneHash); err != nil {
		return err
	}
	ac.ForgetScene(sceneHash)
	return nil
}
