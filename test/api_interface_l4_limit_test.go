package test

import (
	"path/filepath"
	"testing"
	"time"

	memhop "github.com/genosis18m/Long-term-memory-go/api"
)

// `Limit` is documented as keeping the tail of whichever order the query spans — Seq within one topic,
// creation time across topics.
func TestInterfaceL4LimitTruncatesAfterTheFilters(t *testing.T) {
	llm := newMockLLM(t)
	path := filepath.Join(t.TempDir(), "limit.meh")
	m := openMockDB(t, path, llm.srv.URL)
	sess, err := m.Primary()
	if err != nil {
		t.Fatalf("Primary: %v", err)
	}

	base := time.Now().Add(-time.Hour).UnixMilli()
	var topics []string
	for i := 0; i < 2; i++ {
		if _, err := sess.Search(memhop.SearchQuery{NewScene: i == 0}); err != nil {
			t.Fatalf("open round %d: %v", i, err)
		}
		if _, err := sess.AppendArchive(memhop.ArchiveInput{Kind: memhop.KindEvent,
			ContentType: memhop.ContentText, EventType: "tool_call",
			Content: "重试策略查过了", CreatedAt: base + int64(i)*1000}); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
		id, err := sess.Update(memhop.TurnEnd{Input: "问甲", Output: "答甲",
			Outcome: "answered", CreatedAt: base + int64(i)*1000 + 10})
		if err != nil {
			t.Fatalf("close round %d: %v", i, err)
		}
		topics = append(topics, id.ID)
	}

	cases := []struct {
		name  string
		query memhop.L4Query
		limit int
	}{
		{name: "across topics", query: memhop.L4Query{}, limit: 2},
		{name: "one kind across topics", query: memhop.L4Query{Kind: ptr(memhop.KindEvent)}, limit: 2},
		{name: "other kind across topics", query: memhop.L4Query{Kind: ptr(memhop.KindUtterance)}, limit: 3},
		{name: "keyword across topics", query: memhop.L4Query{Keyword: "重试"}, limit: 1},
		{name: "within one topic", query: memhop.L4Query{TopicID: &topics[0]}, limit: 2},
		{name: "topic and kind", query: memhop.L4Query{TopicID: &topics[0], Kind: ptr(memhop.KindEvent)}, limit: 1},
	}
	for _, tc := range cases {
		full := readIDs(t, sess, tc.query)
		if len(full) <= tc.limit {
			t.Fatalf("%s: the fixture gives %d rows for this query, want more than the limit %d so the "+
				"truncation is actually exercised", tc.name, len(full), tc.limit)
		}
		limited := memhop.L4Query{}
		limited = tc.query
		limited.Limit = tc.limit
		got := readIDs(t, sess, limited)
		want := full[len(full)-tc.limit:]
		if !sameSet(got, want) {
			t.Fatalf("%s: a limited read answered %v, want the tail %v of the same query unfiltered (%v) — "+
				"that means truncation ran before the filter", tc.name, got, want, full)
		}
	}
}
