// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// Re-exported data-model surface of the facade: the enum constants and the error contract.

package api

import "github.com/genosis18m/Long-term-memory-go/internal"

// AgentTypePrimary marks the domain a file is opened on — a file holds exactly one of them;
// AgentTypeSub marks a domain created under it.
const (
	AgentTypePrimary = internal.AgentTypePrimary
	AgentTypeSub     = internal.AgentTypeSub
)

const (
	EdgeRelated    = internal.EdgeRelated
	EdgeCausal     = internal.EdgeCausal
	EdgePartOf     = internal.EdgePartOf
	EdgeSequence   = internal.EdgeSequence
	EdgeDependency = internal.EdgeDependency
	EdgeCustom     = internal.EdgeCustom
)

const (
	L3ImportSkip      = internal.L3ImportSkip
	L3ImportMerge     = internal.L3ImportMerge
	L3ImportOverwrite = internal.L3ImportOverwrite
)

// MaxEventPayloadBytes and MaxUtterancePayloadBytes are the per-record write budgets.
const (
	MaxEventPayloadBytes     = internal.MaxEventPayloadBytes
	MaxUtterancePayloadBytes = internal.MaxUtterancePayloadBytes
	// MaxSubAgentNameBytes caps a sub-agent domain name in bytes.
	MaxSubAgentNameBytes = internal.MaxSubAgentNameBytes
)

// RoleUser / RoleAgent / RoleSystem are host-declared utterance roles; RoleDream is library-written.
const (
	RoleUser   = internal.RoleUser
	RoleAgent  = internal.RoleAgent
	RoleSystem = internal.RoleSystem
	RoleDream  = internal.RoleDream
)

// PlanStatus* are the string lifecycle values the plan write surface accepts and PlanState emits.
const (
	PlanStatusInProgress PlanStatus = internal.PlanInProgress
	PlanStatusDone       PlanStatus = internal.PlanDone
	PlanStatusFailed     PlanStatus = internal.PlanFailed
)

// These are the only valid ContentType values.
const (
	ContentText     = internal.ContentText
	ContentImage    = internal.ContentImage
	ContentVideo    = internal.ContentVideo
	ContentDocument = internal.ContentDocument
	ContentAudio    = internal.ContentAudio
	ContentCode     = internal.ContentCode
	ContentOther    = internal.ContentOther
)

// KindUtterance is something somebody said — an original or a Dream-fused summary; KindEvent is
// something that happened while they said it.
const (
	KindUtterance = internal.KindUtterance
	KindEvent     = internal.KindEvent
)
