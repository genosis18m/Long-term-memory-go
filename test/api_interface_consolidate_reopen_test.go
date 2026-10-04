package test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	memhop "github.com/genosis18m/Long-term-memory-go/api"
)

// noAutoDreamButCompress keeps the background trigger off (a Dream scheduled mid-test would move the
// surface under the comparison) while letting the host-driven pass merge.
func noAutoDreamButCompress(d *memhop.MemHopDefaults) {
	d.SceneDreamTopicThreshold = -1
	d.DreamCompressMinTopics = 2
}

// Consolidation is the one pass that rewrites what a scene looks like without deleting anything.
func TestInterfaceConsolidationSurvivesReopen(t *testing.T) {
	llm := newMockLLM(t)
	path := filepath.Join(t.TempDir(), "consolidate.meh")
	m := openMockDB(t, path, llm.srv.URL, noAutoDreamButCompress)
	t.Cleanup(func() { _ = m.Close() })
	sess, err := m.Primary()
	if err != nil {
		t.Fatalf("Primary: %v", err)
	}

	var sceneID string
	stamp := int64(1_700_000_000_000)
	for i := 0; i < 8; i++ {
		res, err := sess.Search(memhop.SearchQuery{NewScene: i == 0})
		if err != nil {
			t.Fatalf("Search %d: %v", i, err)
		}
		sceneID = res.Scene.SceneID
		// Distinct stamps per round: the listing is ordered by them, and a tie would make
		// the order this test compares depend on nothing but luck.
		stamp += 1000
		if _, err := sess.Update(memhop.TurnEnd{Input: "巩固前的提问", Output: "巩固前的回答",
			Outcome: "answered", CreatedAt: stamp}); err != nil {
			t.Fatalf("turn %d: %v", i, err)
		}
	}

	rep, err := sess.Dream(context.Background(), sceneID)
	if err != nil {
		t.Fatalf("Dream: %v", err)
	}
	if rep.L2TopicsCompressed == 0 {
		t.Fatalf("the pass compressed nothing, so the comparison below would be vacuous: %+v", rep)
	}

	dump := func(label string, s *memhop.Session) string {
		s2, err := s.SceneContext(sceneID)
		if err != nil {
			t.Fatalf("%s SceneContext: %v", label, err)
		}
		var groups, sunk int
		for _, row := range s2.Topics {
			if row.ChildCount > 0 {
				groups++
			}
			if row.Depth > 1 {
				sunk++
			}
			if len(row.Keywords) == 0 {
				t.Fatalf("%s: row %s has no keyword track", label, row.TopicID)
			}
		}
		if groups == 0 || sunk == 0 {
			t.Fatalf("%s: the consolidated state is missing from the listing (groups %d, sunk %d)", label, groups, sunk)
		}
		// The documented order is (user timestamp, shallower first, id).
		ties := 0
		for i := 1; i < len(s2.Topics); i++ {
			prev, cur := s2.Topics[i-1], s2.Topics[i]
			if cur.UserTimestamp == prev.UserTimestamp {
				ties++
			}
			if cur.UserTimestamp < prev.UserTimestamp {
				t.Fatalf("%s: rows out of speaking order at %d (%d < %d)", label, i, cur.UserTimestamp, prev.UserTimestamp)
			}
			if cur.UserTimestamp == prev.UserTimestamp && cur.Depth < prev.Depth {
				t.Fatalf("%s: at a timestamp tie the deeper row leads at %d, so a group does not introduce its own originals: %+v then %+v",
					label, i, prev, cur)
			}
			if cur.UserTimestamp == prev.UserTimestamp && cur.Depth == prev.Depth && cur.TopicID < prev.TopicID {
				t.Fatalf("%s: rows sharing timestamp and depth are not id-ordered at %d (%s after %s)", label, i, cur.TopicID, prev.TopicID)
			}
		}
		// Without a tie the two secondary keys above assert nothing, and consolidation
		// guarantees one: a group carries the first swallowed turn's timestamp.
		if ties == 0 {
			t.Fatalf("%s: no two rows shared a timestamp, so the tie-breaking keys went untested", label)
		}
		raw, err := json.Marshal(s2)
		if err != nil {
			t.Fatalf("%s marshal: %v", label, err)
		}
		return string(raw)
	}

	before := dump("before the reopen", sess)
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	m2 := openMockDB(t, path, llm.srv.URL, noAutoDreamButCompress)
	t.Cleanup(func() { _ = m2.Close() })
	sess2, err := m2.Primary()
	if err != nil {
		t.Fatalf("Primary after reopen: %v", err)
	}

	after := dump("after the reopen", sess2)
	if before != after {
		t.Fatalf("the scene read changed across a restart of the same file:\nbefore: %s\nafter:  %s", before, after)
	}
}
