package internal

import (
	"context"
	"testing"

	"github.com/genosis18m/Long-term-memory-go/internal/domain"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// newTestDB wires an engine into the minimal multi-agent DB state the domain-context machinery needs
// (registry, base context, defaults), mirroring the Open assembly.
func newTestDB(t *testing.T, engine *core.StorageEngine) *DB {
	t.Helper()
	baseCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &DB{
		engine:     engine,
		config:     &MemHopConfig{Defaults: DefaultMemHopDefaults},
		baseCtx:    baseCtx,
		baseCancel: cancel,
		agents:     make(map[uint64]*domain.Context),
	}
}

// testDefaultContext returns the default-domain context of db, creating it with caches rebuilt from.
func testDefaultContext(db *DB) *domain.Context {
	if ac := db.agents[core.DefaultAgentID]; ac != nil {
		return ac
	}
	ac := domain.NewContext(core.DefaultAgentID, db.baseCtx, db.engine, db.llm, &db.config.Defaults)
	db.agents[core.DefaultAgentID] = ac
	return ac
}

// TestLoadTenantRegistryDuplicateNameDeterministic pins the restart mapping rule: two active registry
// records carrying the same name resolve to the highest agentID regardless of Go map iteration order.
func TestLoadTenantRegistryDuplicateNameDeterministic(t *testing.T) {
	engine := newTestEngine(t)
	const name = "dup"
	low, high := uint64(0x1111111111111111), uint64(0x2222222222222222)
	for _, id := range []uint64{low, high} {
		if err := repo.WriteAgentRegistry(engine, id, name); err != nil {
			t.Fatalf("write registry record %d: %v", id, err)
		}
	}
	idToName, nameToID, unresolved := loadTenantRegistry(engine)
	if unresolved != nil {
		t.Fatalf("two readable records under one name are not an unreadable key: %v", unresolved)
	}
	if got := nameToID[name]; got != high {
		t.Fatalf("duplicate name resolves to %#x, want higher ID %#x", got, high)
	}
	if idToName[low] != name || idToName[high] != name {
		t.Fatalf("idToName missing entries: %q / %q", idToName[low], idToName[high])
	}
}
