// L0 profile record primitives.
package repo

import (
	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// L0 profile operations: the singleton ProfileSlot at the fixed ID hash("profile") inside the agent
// domain.
func GetProfileL0(engine *core.StorageEngine, agentID uint64) (*core.ProfileSlot, error) {
	slot, err := core.ReadProfileSlot(engine, agentID, common.HashID("profile"))
	if err != nil {
		return nil, common.NewError(common.CodeOf(err), "read profile", err)
	}
	return slot, nil
}

func UpdateProfileL0(engine *core.StorageEngine, agentID uint64, slot *core.ProfileSlot) error {
	id := common.HashID("profile")
	return core.WriteProfileSlot(engine, agentID, id, slot)
}

// HasProfileL0 answers whether a domain's profile record is there, for a caller that acts on presence
// alone.
func HasProfileL0(engine *core.StorageEngine, agentID uint64) (bool, error) {
	_, err := core.ReadProfileSlot(engine, agentID, common.HashID("profile"))
	if err == nil {
		return true, nil
	}
	if common.CodeOf(err) == common.ErrNotFound {
		return false, nil
	}
	return false, err
}
