// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// The public surface of the facade, pinned by reflection: these lists are the review gate, so nothing
// becomes host-callable without an edit here.

package api

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func methodNames(v any) []string {
	t := reflect.TypeOf(v)
	out := make([]string, 0, t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		out = append(out, t.Method(i).Name)
	}
	sort.Strings(out)
	return out
}

func diffNames(want, got []string) (missing, extra []string) {
	have := make(map[string]struct{}, len(got))
	for _, n := range got {
		have[n] = struct{}{}
	}
	wanted := make(map[string]struct{}, len(want))
	for _, n := range want {
		wanted[n] = struct{}{}
	}
	for _, n := range got {
		if _, ok := wanted[n]; !ok {
			extra = append(extra, n)
		}
	}
	for _, n := range want {
		if _, ok := have[n]; !ok {
			missing = append(missing, n)
		}
	}
	return missing, extra
}

func TestSessionPublicSurface(t *testing.T) {
	want := []string{
		// runtime/task face — the host drives these every turn and LLM tools.
		"Search", "Update", "Dream", "AppendArchive",
		// L0 profile.
		"GetL0", "UpdateL0",
		// L1 scene hypergraph (read-only; Dream is the only writer).
		"ListL1",
		// L2 scene reads.
		"ListScenes", "SceneContext",
		// L3 knowledge.
		"GetL3", "ListL3", "ImportL3", "QueryL3Nodes", "QueryL3Subgraph",
		// L4 archives.
		"SearchL4",
		// the plan tree: written one step at a time.
		"PlanNodeAdd", "PlanNodeUpdate", "PlanState",

		// assembly/admin face — host code at session boundaries and management.
		"UpdateScene", "RenameTopic", "MergeScenes", "DeleteScene", "DeleteTopic",
		// L3 management and corrections.
		"UpdateL3", "DeleteL3",
		// the domain's own identity, for handing back to DB.Agent.
		"AgentID",
	}
	sort.Strings(want)

	missing, extra := diffNames(want, methodNames(&Session{}))
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("Session public surface drifted: missing=%v unexpected=%v", missing, extra)
	}
}

func TestDBPublicSurface(t *testing.T) {
	// Three ways in — the domain the file was opened on, one created under it by name, and one addressed.
	want := []string{
		"Primary", "SubAgent", "Agent", "Agents",
		"Checkpoint", "CompactTo", "Close", "IsClosed", "Stats",
	}
	sort.Strings(want)

	missing, extra := diffNames(want, methodNames(&DB{}))
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("DB public surface drifted: missing=%v unexpected=%v", missing, extra)
	}
}

// TestHandlesExposeNoField pins the route the method lists cannot see: an exported field on a handle
// lets a host reach the internal value it holds.
func TestHandlesExposeNoField(t *testing.T) {
	for _, handle := range []struct {
		name string
		v    any
	}{
		{"Session", &Session{}},
		{"DB", &DB{}},
	} {
		typ := reflect.TypeOf(handle.v).Elem()
		for i := 0; i < typ.NumField(); i++ {
			if f := typ.Field(i); f.PkgPath == "" {
				t.Errorf("%s.%s is exported: a host reaches the internal value around every method here",
					handle.name, f.Name)
			}
		}
	}
}

// TestPublicSignaturesCarryNoNumericIds pins the other half of the facade contract: every id a host
// can see is a 16-char hex string.
func TestPublicSignaturesCarryNoNumericIds(t *testing.T) {
	for _, handle := range []struct {
		name string
		v    any
	}{
		{"Session", &Session{}},
		{"DB", &DB{}},
	} {
		typ := reflect.TypeOf(handle.v)
		for i := 0; i < typ.NumMethod(); i++ {
			m := typ.Method(i)
			for side, types := range map[string][]reflect.Type{"param": inTypes(m.Type), "result": outTypes(m.Type)} {
				for _, lt := range types {
					if bad := numericIDField(lt, map[reflect.Type]bool{}); bad != "" {
						t.Errorf("%s.%s exposes %s — render it as a hex string in the facade",
							handle.name, m.Name, side+" "+bad)
					}
				}
			}
		}
	}
}

// TestTurnWritesCarryNoId pins what this round took out of the host's hands.
func TestTurnWritesCarryNoId(t *testing.T) {
	want := map[string]int{
		"Update":         1, // one TurnEnd
		"AppendArchive":  1, // one ArchiveInput
		"PlanNodeAdd":    2, // parentSeq and title
		"PlanNodeUpdate": 1, // one PlanStep
		"PlanState":      0, // nothing at all: the turn is the library's to know
	}
	typ := reflect.TypeOf(&Session{})
	for name, in := range want {
		m, ok := typ.MethodByName(name)
		if !ok {
			t.Fatalf("Session.%s is gone: writing the open turn without naming it is this surface's contract", name)
		}
		if got := len(inTypes(m.Type)); got != in {
			t.Errorf("Session.%s takes %d arguments, want %d — an id has crept back into the host's hands",
				name, got, in)
		}
	}
}

// TestArchiveInputCarriesNoAddress pins the other half of the same contract: the write shape has no
// field for an address the library fills in.
func TestArchiveInputCarriesNoAddress(t *testing.T) {
	for _, banned := range []string{"ID", "TopicID"} {
		if _, ok := reflect.TypeOf(ArchiveInput{}).FieldByName(banned); ok {
			t.Errorf("ArchiveInput carries %s: an address the write path cannot honor", banned)
		}
	}
	if _, ok := reflect.TypeOf(ArchiveSlot{}).FieldByName("TopicID"); !ok {
		t.Fatal("ArchiveSlot lost the address a read names")
	}
}

// inTypes skips In(0): for a method read off a type, that first input is the receiver, whose struct
// graph reaches the storage engine.
func inTypes(fn reflect.Type) []reflect.Type {
	out := make([]reflect.Type, 0, fn.NumIn())
	for i := 1; i < fn.NumIn(); i++ {
		out = append(out, fn.In(i))
	}
	return out
}

func outTypes(fn reflect.Type) []reflect.Type {
	out := make([]reflect.Type, 0, fn.NumOut())
	for i := 0; i < fn.NumOut(); i++ {
		out = append(out, fn.Out(i))
	}
	return out
}

// numericIDField walks a type (and every struct it can reach) looking for a uint64 field named like an
// identifier — the shape an internal record id takes before the facade renders it.
func numericIDField(t reflect.Type, seen map[reflect.Type]bool) string {
	if t == nil || seen[t] {
		return ""
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return numericIDField(t.Elem(), seen)
	case reflect.Map:
		if bad := numericIDField(t.Key(), seen); bad != "" {
			return bad
		}
		return numericIDField(t.Elem(), seen)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue // unexported: a host cannot reach it, embedded handles included
			}
			if f.Type.Kind() == reflect.Uint64 && strings.Contains(f.Name, "ID") {
				return t.Name() + "." + f.Name + " is a uint64 id"
			}
			if bad := numericIDField(f.Type, seen); bad != "" {
				return t.Name() + "." + bad
			}
		}
	}
	return ""
}
