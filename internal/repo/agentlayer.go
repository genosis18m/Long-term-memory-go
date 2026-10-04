// Agent tenant registry records: one RecAgentRegistry frame per agent maps the random 8-byte agentID
// to its external name.

package repo

import (
	"encoding/json"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// WriteAgentRegistry upserts the tenant registration record of one agent.
func WriteAgentRegistry(engine *core.StorageEngine, agentID uint64, name string) error {
	data, err := json.Marshal(name)
	if err != nil {
		return common.NewError(common.ErrSerialization, "agent registry", err)
	}
	_, err = engine.WriteRecord(agentID, core.RecAgentRegistry, agentID, data)
	return err
}

// ListAgentRegistry scans every domain's registry records and returns agentID -> name, plus the
// failure of the first record that exists but resolves to no name (unreadable, undecodable, or empty).
func ListAgentRegistry(engine *core.StorageEngine) (map[uint64]string, error) {
	out := make(map[uint64]string)
	var unresolved error
	for agentID := range engine.IterAgents() {
		for idHash := range engine.IndexByType(agentID, core.RecAgentRegistry) {
			_, data, err := engine.ReadRecord(agentID, idHash)
			var name string
			if err == nil {
				if uerr := json.Unmarshal(data, &name); uerr != nil {
					err = common.NewError(common.ErrDeserialization, "unmarshal tenant key", uerr)
				} else if name == "" {
					err = common.NewError(common.ErrDeserialization, "the key is empty")
				}
			}
			if err != nil {
				if unresolved == nil {
					unresolved = common.NewError(common.CodeOf(err),
						"agent registry: domain "+common.FormatHash(agentID)+" carries no readable tenant key", err)
				}
				continue
			}
			out[agentID] = name
		}
	}
	return out, unresolved
}
