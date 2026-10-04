// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

package api

import (
	"regexp"
	"testing"
)

// The vocabularies on this surface come in two wire forms: four enums travel as their
// numbers (content medium, archive kind, utterance speaker, L3 edge kind) and two as words
// (a plan step's status, an import's conflict mode).

var snakeKey = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func TestStringEnumsSpellTheirValuesLowercase(t *testing.T) {
	for _, pair := range []struct {
		kind  string
		value string
	}{
		{"PlanStatus", string(PlanStatusInProgress)},
		{"PlanStatus", string(PlanStatusDone)},
		{"PlanStatus", string(PlanStatusFailed)},
		{"L3ImportMode", string(L3ImportSkip)},
		{"L3ImportMode", string(L3ImportMerge)},
		{"L3ImportMode", string(L3ImportOverwrite)},
	} {
		if !snakeKey.MatchString(pair.value) {
			t.Errorf("%s carries the value %q: every string value on this surface is lowercase with underscores", pair.kind, pair.value)
		}
	}
}

func TestEnumVocabulariesMatchWireContract(t *testing.T) {
	type row struct {
		name   string
		value  string // how it reads through String()
		number string // its numeric value, for the three numeric enums
	}
	tables := map[string][]row{
		"ContentType": {
			{"ContentText", "text", "0"}, {"ContentImage", "image", "1"}, {"ContentVideo", "video", "2"},
			{"ContentDocument", "document", "3"}, {"ContentAudio", "audio", "4"}, {"ContentCode", "code", "5"},
			{"ContentOther", "other", "255"},
		},
		"ArchiveKind": {
			{"KindUtterance", "utterance", "0"}, {"KindEvent", "event", "1"},
		},
		"ArchiveRole": {
			{"RoleUser", "user", "0"}, {"RoleAgent", "agent", "1"}, {"RoleSystem", "system", "2"},
			{"RoleDream", "dream", "3"},
		},
		"GraphEdgeKind": {
			{"EdgeRelated", "related", "0"}, {"EdgeCausal", "causal", "1"}, {"EdgePartOf", "part_of", "2"},
			{"EdgeSequence", "sequence", "3"}, {"EdgeDependency", "dependency", "4"}, {"EdgeCustom", "custom", "5"},
		},
	}
	for enum, rows := range tables {
		for _, r := range rows {
			got := enumValue(t, enum, r.name)
			if got.String() != r.value {
				t.Errorf("%s.%s prints %q, want %q", enum, r.name, got.String(), r.value)
			}
			if !got.Valid() {
				t.Errorf("%s.%s reports itself undefined, so the vocabulary table and Valid() disagree", enum, r.name)
			}
			wire := encode(t, enum+"."+r.name, got)
			if wire != r.number {
				t.Errorf("%s.%s encodes as %s, want the number %s a tool schema has to promise",
					enum, r.name, wire, r.number)
			}
		}
		if last := enumMax(t, enum); last.Valid() {
			t.Errorf("%s still validates one past its defined set (%v)", enum, last)
		}
	}
}

func enumValue(tb testing.TB, enum, name string) interface {
	String() string
	Valid() bool
} {
	tb.Helper()
	switch enum + "." + name {
	case "ContentType.ContentText":
		return ContentText
	case "ContentType.ContentImage":
		return ContentImage
	case "ContentType.ContentVideo":
		return ContentVideo
	case "ContentType.ContentDocument":
		return ContentDocument
	case "ContentType.ContentAudio":
		return ContentAudio
	case "ContentType.ContentCode":
		return ContentCode
	case "ContentType.ContentOther":
		return ContentOther
	case "ArchiveKind.KindUtterance":
		return KindUtterance
	case "ArchiveKind.KindEvent":
		return KindEvent
	case "ArchiveRole.RoleUser":
		return RoleUser
	case "ArchiveRole.RoleAgent":
		return RoleAgent
	case "ArchiveRole.RoleSystem":
		return RoleSystem
	case "ArchiveRole.RoleDream":
		return RoleDream
	case "GraphEdgeKind.EdgeRelated":
		return EdgeRelated
	case "GraphEdgeKind.EdgeCausal":
		return EdgeCausal
	case "GraphEdgeKind.EdgePartOf":
		return EdgePartOf
	case "GraphEdgeKind.EdgeSequence":
		return EdgeSequence
	case "GraphEdgeKind.EdgeDependency":
		return EdgeDependency
	case "GraphEdgeKind.EdgeCustom":
		return EdgeCustom
	}
	tb.Fatalf("no such constant in the table: %s.%s", enum, name)
	return nil
}

func enumMax(tb testing.TB, enum string) interface {
	String() string
	Valid() bool
} {
	tb.Helper()
	switch enum {
	case "ContentType":
		return ContentType(6)
	case "ArchiveKind":
		return ArchiveKind(2)
	case "ArchiveRole":
		return ArchiveRole(4)
	case "GraphEdgeKind":
		return GraphEdgeKind(6)
	}
	tb.Fatalf("no top value defined for %s", enum)
	return nil
}
