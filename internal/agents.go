// Agent domain management of the internal layer.

package internal

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// ensureRegistered returns the stable agentID for name, allocating a fresh crypto/rand ID (and writing
// its registry record) on first use.
func (db *DB) ensureRegistered(name string) (uint64, error) {
	if db.closed.Load() {
		return 0, errDBClosed
	}
	db.agentsMu.Lock()
	defer db.agentsMu.Unlock()
	if db.nameToID == nil {
		db.nameToID = make(map[string]uint64)
		db.idToName = make(map[uint64]string)
	}
	if id, ok := db.nameToID[name]; ok {
		return id, nil
	}
	// A name is only free while no domain is holding a key that will not resolve.
	if _, unresolved := repo.ListAgentRegistry(db.engine); unresolved != nil {
		return 0, unresolved
	}
	for {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return 0, common.NewError(common.ErrIO, "agent id allocation", err)
		}
		id := binary.LittleEndian.Uint64(b[:])
		if id == core.DefaultAgentID || id == core.SharedPoolAgentID {
			continue
		}
		if _, taken := db.idToName[id]; taken {
			continue
		}
		if err := repo.WriteAgentRegistry(db.engine, id, name); err != nil {
			return 0, err
		}
		db.nameToID[name] = id
		db.idToName[id] = name
		return id, nil
	}
}

// HasAgent reports whether agentID is the default domain or a registered tenant.
func (db *DB) HasAgent(agentID uint64) bool {
	if agentID == core.DefaultAgentID {
		return true
	}
	db.agentsMu.Lock()
	defer db.agentsMu.Unlock()
	_, ok := db.idToName[agentID]
	return ok
}

// Primary returns the session bound to the domain the file was opened on — the implicit zero one, so a
// file holds exactly one primary.
func (db *DB) Primary() (*Session, error) {
	return db.NewSession(core.DefaultAgentID)
}

// MaxSubAgentNameBytes caps a tenant key.
const MaxSubAgentNameBytes = 256

// SubAgent returns the session of the sub-agent domain named profile.Name, creating that domain the
// first time and handing back the same one after.
func (db *DB) SubAgent(llmCfg LlmConfig, profile core.ProfileSlot) (*Session, error) {
	if err := llmCfg.Validate(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		return nil, common.NewError(common.ErrInvalidQuery, "sub-agent profile Name is required")
	}
	if len(name) > MaxSubAgentNameBytes {
		return nil, common.NewError(common.ErrInvalidQuery,
			fmt.Sprintf("sub-agent name exceeds %d bytes", MaxSubAgentNameBytes))
	}
	id, err := db.ensureRegistered(name)
	if err != nil {
		return nil, err
	}
	provider := db.setDomainLLM(id, llmCfg)
	// Session admission reads the registry, so the handle has to be fetched before the domain lock is
	// taken: agentsMu under ac.Mu is the one lock order this layer must never build.
	sess, err := db.NewSession(id)
	if err != nil {
		return nil, err
	}
	ac, err := db.lockAgent(id)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	// contextFor only reads the table when building a context, so a live one has to be re-pointed here —
	// under the domain lock, which is where every operation reads the transport.
	ac.LLM = provider
	has, err := repo.HasProfileL0(db.engine, id)
	if err != nil {
		return nil, err
	}
	if has {
		return sess, nil
	}
	slot := profile
	slot.Name = name
	slot.AgentType = core.AgentTypeSub
	slot.UpdatedAtMs = time.Now().UnixMilli()
	if err := repo.UpdateProfileL0(db.engine, id, &slot); err != nil {
		return nil, err
	}
	return sess, nil
}

// Agent returns the session of a domain this file already holds, addressed by the id the library
// handed out for it.
func (db *DB) Agent(llmCfg LlmConfig, agentIDHex string) (*Session, error) {
	if err := llmCfg.Validate(); err != nil {
		return nil, err
	}
	id, err := parseID("agent", agentIDHex)
	if err != nil {
		return nil, err
	}
	if err := db.CheckSession(id); err != nil {
		return nil, err
	}
	provider := db.setDomainLLM(id, llmCfg)
	sess, err := db.NewSession(id)
	if err != nil {
		return nil, err
	}
	ac, err := db.lockAgent(id)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	// Same reason as in SubAgent: a live context caches the transport, and the lock is
	// what makes that write and every read of it agree.
	ac.LLM = provider
	return sess, nil
}

// AgentInfo is one domain of this file as Agents lists it: the id the library issued, the name that
// keys the domain, and whether it is the primary.
type AgentInfo struct {
	AgentID uint64
	Name    string
	Primary bool
}

// Agents lists every domain the file holds.
func (db *DB) Agents() ([]AgentInfo, error) {
	if db.closed.Load() {
		return nil, errDBClosed
	}
	names, unresolved := repo.ListAgentRegistry(db.engine)
	if unresolved != nil {
		return nil, unresolved
	}
	primary, err := db.GetL0(core.DefaultAgentID)
	if err != nil {
		return nil, err
	}
	out := make([]AgentInfo, 0, len(names)+1)
	out = append(out, AgentInfo{AgentID: core.DefaultAgentID, Name: primary.Name, Primary: true})
	for _, id := range slices.Sorted(maps.Keys(names)) {
		out = append(out, AgentInfo{AgentID: id, Name: names[id]})
	}
	return out, nil
}

// CheckSession is the session-eligibility policy: the database must be open and agentID must address a
// registered tenant or the default domain.
func (db *DB) CheckSession(agentID uint64) error {
	if db.closed.Load() {
		return errDBClosed
	}
	if !db.HasAgent(agentID) {
		return common.NewError(common.ErrAgentNotFound, "unknown agent: "+common.FormatHash(agentID))
	}
	return nil
}
