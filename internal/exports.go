// Re-export seam for the api facade.

package internal

import (
	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/config"
	"github.com/genosis18m/Long-term-memory-go/internal/content"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

type (
	MemHopConfig   = config.MemHopConfig
	LlmConfig      = config.LlmConfig
	MemHopDefaults = config.MemHopDefaults
)

// DefaultMemHopDefaults is the shared default engine configuration.
var DefaultMemHopDefaults = config.DefaultMemHopDefaults

type (
	ProfileSlot    = core.ProfileSlot
	SceneSlot      = core.SceneSlot
	SceneNode      = core.SceneNode
	HypergraphSlot = core.HypergraphSlot
	HypergraphNode = core.HypergraphNode
	HypergraphEdge = core.HypergraphEdge
	ArchiveSlot    = core.ArchiveSlot
	ArchiveKind    = core.ArchiveKind
	ArchiveRole    = core.ArchiveRole
	GraphEdgeKind  = core.GraphEdgeKind
	TopicSlot      = core.TopicSlot
	ContentType    = core.ContentType
)

// NewError re-exported so the api facade can build domain errors without importing internal/common.
var NewError = common.NewError

// Code is the numeric error-code type carried inside Error.
type Code = common.Code

// CodeOf extracts the numeric error code of err (0 when it is not a MemHop Error).
func CodeOf(err error) Code { return common.CodeOf(err) }

// Error codes; see internal/common/errors.go for the interval contract.
const (
	ErrConfig          = common.ErrConfig
	ErrInvalidQuery    = common.ErrInvalidQuery
	ErrNotFound        = common.ErrNotFound
	ErrAgentNotFound   = common.ErrAgentNotFound
	ErrIO              = common.ErrIO
	ErrClosed          = common.ErrClosed
	ErrInvalidMagic    = common.ErrInvalidMagic
	ErrCRCMismatch     = common.ErrCRCMismatch
	ErrCorruption      = common.ErrCorruption
	ErrSerialization   = common.ErrSerialization
	ErrDeserialization = common.ErrDeserialization
	ErrCancelled       = common.ErrCancelled
	ErrLLM             = common.ErrLLM
)

const (
	EdgeRelated    = core.EdgeRelated
	EdgeCausal     = core.EdgeCausal
	EdgePartOf     = core.EdgePartOf
	EdgeSequence   = core.EdgeSequence
	EdgeDependency = core.EdgeDependency
	EdgeCustom     = core.EdgeCustom
)

const (
	RoleUser   = core.RoleUser
	RoleAgent  = core.RoleAgent
	RoleSystem = core.RoleSystem
	// RoleDream 是库给巩固组摘要自己戳的记号：只出得去（SceneContext 与 SearchL4 会带回来）， 进不来——content.ValidateAppend 拒任何带它的写入。.
	RoleDream = core.RoleDream
)

// The two payloads a host may write per record.
const (
	MaxEventPayloadBytes     = content.MaxEventPayload
	MaxUtterancePayloadBytes = content.MaxUtterancePayload
)

const (
	AgentTypePrimary = core.AgentTypePrimary
	AgentTypeSub     = core.AgentTypeSub
)

const (
	KindUtterance = core.KindUtterance
	KindEvent     = core.KindEvent
)

const (
	ContentText     = core.ContentText
	ContentImage    = core.ContentImage
	ContentVideo    = core.ContentVideo
	ContentDocument = core.ContentDocument
	ContentAudio    = core.ContentAudio
	ContentCode     = core.ContentCode
	ContentOther    = core.ContentOther
)

// FormatID renders any record or domain ID as its external 16-char hex form — the only id shape the
// facade exchanges with a host, since every id is issued by the library (Search mints turn ids).
func FormatID(id uint64) string { return common.FormatHash(id) }

// parseID is that boundary's other direction: it reads one host-supplied hex id back into the numeric
// form, naming which field it came from.
func parseID(field, hexID string) (uint64, error) {
	id, err := common.ParseID(hexID)
	if err != nil {
		return 0, common.NewError(common.ErrInvalidQuery, "parse "+field+" id", err)
	}
	return id, nil
}
