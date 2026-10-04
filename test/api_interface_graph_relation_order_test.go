// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// Acceptance item 4 from the side the host writes from.

package test

import (
	"path/filepath"
	"testing"

	memhop "github.com/genosis18m/Long-term-memory-go/api"
)

func TestInterfaceRelationIdentityIgnoresOrderAndAnchor(t *testing.T) {
	llm := newMockLLM(t)
	path := filepath.Join(t.TempDir(), "rel_order.meh")
	m := openMockDB(t, path, llm.srv.URL)
	db := newTestDB(t, m)

	node := func(title, content string) memhop.L3ImportItem {
		return memhop.L3ImportItem{Title: title, Domain: "proj", NodeType: "file", Content: content}
	}

	// One fact, stated from A: {A,B,C} related by dependency.
	res, err := db.ImportL3([]memhop.L3ImportItem{
		{Title: "A", Domain: "proj", NodeType: "file", Content: "the caller",
			Related: []memhop.L3Relation{{Titles: []string{"B", "C"}, Kind: memhop.EdgeDependency}}},
		node("B", "the middle"), node("C", "the leaf"),
	}, memhop.L3ImportOverwrite)
	if err != nil {
		t.Fatalf("seed the graph: %v", err)
	}
	if len(res.GraphIDs) != 1 || res.EdgesCreated != 1 || len(res.Errors) != 0 {
		t.Fatalf("first import: %+v, want one graph, one edge, no complaints", res)
	}
	graphID := res.GraphIDs[0]
	graph := func() *memhop.L3Graph {
		g, err := db.GetL3(graphID)
		if err != nil {
			t.Fatalf("GetL3: %v", err)
		}
		return g
	}

	// The same fact restated from B, with the members in another order — and, in the same batch, a
	// relation that really is new.
	res, err = db.ImportL3([]memhop.L3ImportItem{
		{Title: "B", Domain: "proj", NodeType: "file", Content: "the middle",
			Related: []memhop.L3Relation{
				{Titles: []string{"C", "A"}, Kind: memhop.EdgeDependency},
				{Titles: []string{"C"}, Kind: memhop.EdgePartOf},
			}},
	}, memhop.L3ImportOverwrite)
	if err != nil {
		t.Fatalf("restate from B: %v", err)
	}
	if res.EdgesCreated != 1 || len(res.Errors) != 0 {
		t.Fatalf("the same set with the same kind is the same fact: %+v, want exactly the new kind to land", res)
	}
	if got := edgeSets(graph()); got != "2:{B,C} 4:{A,B,C}" {
		t.Fatalf("edges after restating from another member: %q", got)
	}

	// A different member set under the same kind is a different fact and must not be swallowed
	// by the dedup.
	if res, err = db.ImportL3([]memhop.L3ImportItem{
		{Title: "C", Domain: "proj", NodeType: "file", Content: "the leaf",
			Related: []memhop.L3Relation{{Titles: []string{"A"}, Kind: memhop.EdgeDependency}}},
	}, memhop.L3ImportOverwrite); err != nil || res.EdgesCreated != 1 {
		t.Fatalf("a two-member set under the same kind: %+v (err %v), want its own edge", res, err)
	}
	if got, want := edgeSets(graph()), "2:{B,C} 4:{A,B,C} 4:{A,C}"; got != want {
		t.Fatalf("edges: %q, want %q", got, want)
	}

	// The dedup has to hold across a restart, where the batch's in-memory set is empty and the
	// only thing saying "this fact is already here" is the record on disk.
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	m2 := openMockDB(t, path, llm.srv.URL)
	db2 := newTestDB(t, m2)
	res, err = db2.ImportL3([]memhop.L3ImportItem{
		{Title: "A", Domain: "proj", NodeType: "file", Content: "the caller",
			Related: []memhop.L3Relation{{Titles: []string{"C", "B"}, Kind: memhop.EdgeDependency}}},
	}, memhop.L3ImportOverwrite)
	if err != nil {
		t.Fatalf("restate after restart: %v", err)
	}
	if res.EdgesCreated != 0 || len(res.Errors) != 0 {
		t.Fatalf("the reopened file invented a second copy of a fact it already held: %+v", res)
	}
	g2, err := db2.GetL3(graphID)
	if err != nil {
		t.Fatalf("GetL3 after restart: %v", err)
	}
	if got, want := edgeSets(g2), "2:{B,C} 4:{A,B,C} 4:{A,C}"; got != want {
		t.Fatalf("the graph the host reads back after restart: %q, want %q", got, want)
	}
}
