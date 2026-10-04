// Search of the composition root: a scene-scoped read of the host's own session plus the turn it
// opens.

package internal

import (
	"github.com/genosis18m/Long-term-memory-go/internal/cap/profile"
	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/domain"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
	"github.com/genosis18m/Long-term-memory-go/internal/scene"
	"github.com/genosis18m/Long-term-memory-go/internal/turn"
	"time"
)

// Search reads the domain's conversation and opens the turn the host is about to run.
func (db *DB) Search(agentID uint64, q SearchQuery) (*SearchResult, error) {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()

	// One stamp per read: the scene this read opens a turn on, and any scene it creates, carry the same.
	stamp := ac.NextUsedStamp(time.Now().UnixMilli())
	sceneID, err := db.resolveScene(ac, agentID, q, stamp)
	if err != nil {
		return nil, err
	}
	sceneSlot, err := repo.OpenSceneTurn(db.engine, agentID, sceneID, stamp)
	if err != nil {
		return nil, err
	}
	slot, err := turn.ReadProfile(db.engine, agentID)
	if err != nil {
		return nil, err
	}
	topics := scene.SurfaceTopics(ac, sceneSlot.SceneID)
	opened := core.ComputeTurnTopicID(sceneSlot.SceneID, sceneSlot.TurnSeq)
	if opened == 0 {
		// Zero is what the domain uses to say "no turn is open", so a turn key that hashes to it could never
		// be told apart from a read that never happened.
		return nil, common.NewError(common.ErrCorruption,
			"the turn key this scene allocated is the reserved zero value")
	}
	ac.Scene, ac.Turn = sceneSlot.SceneID, opened
	return &SearchResult{
		Profile:      slot,
		ProfileBrief: profile.Brief(slot),
		Scene:        *sceneSlot,
		Topics:       topics,
		NewTopicID:   opened,
	}, nil
}

// resolveScene reads the host's query into the scene this read is scoped to.
func (db *DB) resolveScene(ac *domain.Context, agentID uint64, q SearchQuery, stamp int64) (uint64, error) {
	if q.SceneID != "" {
		if q.NewScene {
			// The two flags ask for opposite things.
			return 0, common.NewError(common.ErrInvalidQuery,
				"scene_id names a conversation to continue and new_scene asks for a fresh one: pass one or the other")
		}
		named, err := parseID("scene", q.SceneID)
		if err != nil {
			return 0, err
		}
		return scene.ResolveExisting(db.engine, agentID, named, q.L3ID != "")
	}
	anchor, err := sceneAnchor(q.L3ID)
	if err != nil {
		return 0, err
	}
	if q.NewScene {
		return scene.Create(db.engine, agentID, anchor, stamp)
	}
	if err := db.ensureScene(ac, agentID); err != nil {
		return 0, err
	}
	if ac.Scene == 0 {
		return scene.Create(db.engine, agentID, anchor, stamp)
	}
	if anchor != 0 {
		return 0, common.NewError(common.ErrInvalidQuery,
			"an L3 anchor is set when a scene is created: pass NewScene to hang a new conversation on a project, or UpdateScene to move the current one")
	}
	return ac.Scene, nil
}

// ensureScene fills the domain's memory of which scene it is working from the records, when nothing
// holds it: the first read after an open, an idle sweep, a delete or a merge.
func (db *DB) ensureScene(ac *domain.Context, agentID uint64) error {
	if ac.Scene != 0 {
		return nil
	}
	id, err := scene.CurrentScene(db.engine, agentID)
	ac.Scene = id
	return err
}

// readScene resolves the scene a write-free read is scoped to: the id the host named, or the domain's
// own current one when it names none.
func (db *DB) readScene(ac *domain.Context, agentID uint64, sceneID string) (uint64, bool, error) {
	if sceneID != "" {
		id, err := parseID("scene", sceneID)
		return id, true, err
	}
	if err := db.ensureScene(ac, agentID); err != nil {
		return 0, false, err
	}
	if ac.Scene == 0 {
		return 0, false, nil
	}
	return ac.Scene, true, nil
}

// sceneAnchor parses the project domain a created scene hangs on; an empty string is no anchor, and a
// named one that does not parse is refused before any scene is allocated.
func sceneAnchor(l3ID string) (uint64, error) {
	if l3ID == "" {
		return 0, nil
	}
	return parseID("l3", l3ID)
}
