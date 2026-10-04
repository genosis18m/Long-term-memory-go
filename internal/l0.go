// L0 profile operations of the internal layer: the read shares the turn package's profile read, the
// write is a thin wrapper over the repo layer.

package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
	"github.com/genosis18m/Long-term-memory-go/internal/turn"
)

// GetL0 reads the profile singleton of one agent.
func (db *DB) GetL0(agentID uint64) (*core.ProfileSlot, error) {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	slot, err := turn.ReadProfile(db.engine, agentID)
	if err != nil {
		return nil, err
	}
	return &slot, nil
}

// UpdateL0 writes the host-owned half of the profile (Name/Role/Personality/ Preferences).
func (db *DB) UpdateL0(agentID uint64, slot *core.ProfileSlot) error {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return err
	}
	defer ac.Mu.Unlock()
	if strings.TrimSpace(slot.Name) == "" {
		return common.NewError(common.ErrInvalidQuery, "UpdateL0: profile Name is required")
	}
	cur, err := repo.GetProfileL0(db.engine, agentID)
	if err != nil {
		if common.CodeOf(err) != common.ErrNotFound {
			return err
		}
	} else {
		// A sub-domain's Name is the key its door is filed under: SubAgent resolves a name through the
		// registry, which this write does not touch.
		if cur.AgentType == core.AgentTypeSub && slot.Name != cur.Name {
			return common.NewError(common.ErrInvalidQuery,
				fmt.Sprintf("UpdateL0: this sub-agent is registered as %q; its Name is the handle SubAgent opens it by, not a field to rewrite", cur.Name))
		}
		slot.EmotionState = cur.EmotionState
		slot.MBTI = cur.MBTI
		slot.AgentType = cur.AgentType
	}
	slot.UpdatedAtMs = time.Now().UnixMilli()
	return repo.UpdateProfileL0(db.engine, agentID, slot)
}
