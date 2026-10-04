package test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	memhop "github.com/genosis18m/Long-term-memory-go/api"
)

// `DB.Stats` is the only diagnostic a host has, and the decision it is read for — whether to compact —
// is a decision about the log, not about the caches.
func TestInterfaceDeletedRecordsStayDeletedAcrossReopen(t *testing.T) {
	cases := []struct {
		name string
		// prep runs between the seed and the correction, to set up a case that needs a second
		// thing on the file to act on. The baseline census is taken after it.
		prep   func(tb testing.TB, sess *memhop.Session) error
		mutate func(tb testing.TB, sess *memhop.Session, s transcript) error
		// reclaims is the expectation on the census: the correction writes nothing but what
		// it takes away, so the records must fall and the file must grow.
		reclaims bool
	}{
		{name: "nothing changed", mutate: func(tb testing.TB, sess *memhop.Session, s transcript) error { return nil }},
		{name: "delete topic", reclaims: true, mutate: func(tb testing.TB, sess *memhop.Session, s transcript) error {
			return sess.DeleteTopic(s.topics[0])
		}},
		{name: "delete scene", reclaims: true, mutate: func(tb testing.TB, sess *memhop.Session, s transcript) error {
			return sess.DeleteScene(s.sceneID)
		}},
		{name: "merge scenes", reclaims: true, prep: func(tb testing.TB, sess *memhop.Session) error {
			if _, err := sess.Search(memhop.SearchQuery{NewScene: true}); err != nil {
				return err
			}
			_, err := turn(sess, "第二会话的一问", "第二会话的一答")
			return err
		}, mutate: func(tb testing.TB, sess *memhop.Session, s transcript) error {
			return sess.MergeScenes(s.sceneID, []string{mergedAwayScene(tb, sess, s.sceneID)})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			llm := newMockLLM(t)
			path := filepath.Join(t.TempDir(), "reconcile.meh")
			m := openMockDB(t, path, llm.srv.URL)
			sess, err := m.Primary()
			if err != nil {
				t.Fatalf("Primary: %v", err)
			}
			s := seedTranscript(t, sess)
			if tc.prep != nil {
				if err := tc.prep(t, sess); err != nil {
					t.Fatalf("prepare %s: %v", tc.name, err)
				}
			}
			baseBytes, baseRecords, err := census(m)
			if err != nil {
				t.Fatalf("census the file to correct: %v", err)
			}
			if baseRecords == 0 {
				t.Fatal("the seeded transcript reports no records: the numbers below compare nothing")
			}
			if err := tc.mutate(t, sess, s); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			liveBytes, liveRecords, err := census(m)
			if err != nil {
				t.Fatalf("census after the change: %v", err)
			}
			if !tc.reclaims {
				if liveRecords != baseRecords || liveBytes != baseBytes {
					t.Fatalf("a pass with no correction moved the census: %d records in %d bytes -> %d in %d",
						baseRecords, baseBytes, liveRecords, liveBytes)
				}
			} else {
				if liveRecords >= baseRecords {
					t.Fatalf("%s reclaimed nothing: %d records before it, %d after", tc.name, baseRecords, liveRecords)
				}
				if liveBytes <= baseBytes {
					t.Fatalf("%s dropped %d records without the file growing (%d -> %d bytes): the tombstones "+
						"never reached the log, so a snapshot that stops loading rebuilds them as live",
						tc.name, baseRecords-liveRecords, baseBytes, liveBytes)
				}
			}
			if err := m.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			again := openMockDB(t, path, llm.srv.URL)
			_, reopened, err := census(again)
			if err != nil {
				t.Fatalf("count after reopen: %v", err)
			}
			if reopened != liveRecords {
				t.Fatalf("%s: the live instance counts %d records, reopening the same file counts %d — "+
					"the change reached the index but not the log, so what the host deleted is back",
					tc.name, liveRecords, reopened)
			}
		})
	}
}

// The retention sweep is the other way records leave, and the census cannot witness it the same way.
func TestInterfaceSweptRecordsStaySweptAcrossReopen(t *testing.T) {
	llm := newMockLLM(t)
	path := filepath.Join(t.TempDir(), "swept.meh")
	m := openMockDB(t, path, llm.srv.URL, sweepOnly)
	sess, err := m.Primary()
	if err != nil {
		t.Fatalf("Primary: %v", err)
	}
	seedTranscript(t, sess)
	// The window is a millisecond wide, so everything written above is past it by now.
	time.Sleep(20 * time.Millisecond)
	rep, err := sess.Dream(context.Background(), "")
	if err != nil {
		t.Fatalf("Dream: %v", err)
	}
	if rep.L4RecordsPruned != 4 {
		t.Fatalf("the pass swept %d content records, want the four the two seeded turns own: %+v",
			rep.L4RecordsPruned, rep)
	}
	if rep.L5NodesPruned != 1 {
		t.Fatalf("the pass swept %d plan nodes, want the one step the second turn carries: %+v",
			rep.L5NodesPruned, rep)
	}
	_, live, err := census(m)
	if err != nil {
		t.Fatalf("census after the sweep: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	again := openMockDB(t, path, llm.srv.URL, sweepOnly)
	_, reopened, err := census(again)
	if err != nil {
		t.Fatalf("census after reopen: %v", err)
	}
	if reopened != live {
		t.Fatalf("the sweep left the file counting %d records against the live %d: the swept records are "+
			"still in the log, and the next pass pays to sweep them again", live, reopened)
	}
}

// transcript is what one seeded scene owns: its id and the ids of the turns that closed under it,
// which are the addresses the delete paths take.
type transcript struct {
	sceneID string
	topics  []string
}

// mergedAwayScene reads back the scene the merge case just opened.
func mergedAwayScene(tb testing.TB, sess *memhop.Session, survivor string) string {
	tb.Helper()
	scenes, err := sess.ListScenes("")
	if err != nil {
		tb.Fatalf("ListScenes: %v", err)
	}
	if len(scenes) != 2 {
		tb.Fatalf("the merge case needs two scenes, the listing has %d: %+v", len(scenes), scenes)
	}
	for _, sc := range scenes {
		if sc.SceneID != survivor {
			return sc.SceneID
		}
	}
	tb.Fatalf("the survivor %s is the whole listing: %+v", survivor, scenes)
	return ""
}

// seedTranscript opens one scene and closes two turns under it, the second turn carrying a plan step,
// so every path in these tables has a record of its own to remove.
func seedTranscript(tb testing.TB, sess *memhop.Session) transcript {
	tb.Helper()
	first, err := sess.Search(memhop.SearchQuery{NewScene: true})
	if err != nil {
		tb.Fatalf("open scene: %v", err)
	}
	s := transcript{sceneID: first.Scene.SceneID}
	one, err := turn(sess, "第一问", "第一答")
	if err != nil {
		tb.Fatalf("close turn 1: %v", err)
	}
	s.topics = append(s.topics, one)
	if _, err := sess.Search(memhop.SearchQuery{}); err != nil {
		tb.Fatalf("open turn 2: %v", err)
	}
	step, err := sess.PlanNodeAdd(0, "查一次资料")
	if err != nil {
		tb.Fatalf("add step: %v", err)
	}
	if err := sess.PlanNodeUpdate(memhop.PlanStep{Seq: step, Status: memhop.PlanStatusDone, Summary: "查到了"}); err != nil {
		tb.Fatalf("settle step: %v", err)
	}
	two, err := turn(sess, "第二问", "第二答")
	if err != nil {
		tb.Fatalf("close turn 2: %v", err)
	}
	s.topics = append(s.topics, two)
	return s
}

// census reads the two numbers the facade reports together, because which record is live and how much
// log it costs are the two halves of every decision about compaction.
func census(m *memhop.DB) (bytes, records int64, err error) {
	st, err := m.Stats()
	if err != nil {
		return 0, 0, err
	}
	return st.FileBytes, st.RecordCount, nil
}

// sweepOnly leaves a pass one stage to run.
func sweepOnly(d *memhop.MemHopDefaults) {
	d.SceneDreamTopicThreshold = -1
	d.DreamCompressMinTopics = 100
	d.ContentRetentionMs = 1
}
