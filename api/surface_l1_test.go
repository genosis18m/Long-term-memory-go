// L1 read surface: the ids a host gets are hex like every other id, and the values are Dream's — there
// is no write to test here.

package api

import (
	"context"
	"testing"
)

// A domain nobody consolidated yet has no nodes.
func TestSurfaceListL1OnAnUndreamedDomain(t *testing.T) {
	db := openSurfaceDB(t)
	nodes, err := db.ListL1()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if nodes == nil {
		t.Fatal("want an empty non-nil list, got nil")
	}
	if len(nodes) != 0 {
		t.Fatalf("an undreamed domain reported %d nodes", len(nodes))
	}
}

// Dream's node sync needs no LLM, so two scenes and one pass are enough to get real nodes through the
// facade: every id crosses as 16-hex, and the node points back at the scene it was built from.
func TestSurfaceListL1RendersHexIDs(t *testing.T) {
	db := openSurfaceDB(t)
	first, err := db.Search(SearchQuery{})
	if err != nil {
		t.Fatalf("search scene one: %v", err)
	}
	// Nodes are synced from a scene's topics, so a scene that never settled a turn has nothing to build
	// one out of.
	if _, err := settleTurn(db, "第一个会话聊了什么", "记下了"); err != nil {
		t.Fatalf("settle scene one: %v", err)
	}
	second, err := db.Search(SearchQuery{NewScene: true})
	if err != nil {
		t.Fatalf("search scene two: %v", err)
	}
	if second.Scene.SceneID == first.Scene.SceneID {
		t.Fatalf("NewScene continued the domain's current scene: %s", first.Scene.SceneID)
	}
	if _, err := settleTurn(db, "第二个会话聊了什么", "也记下了"); err != nil {
		t.Fatalf("settle scene two: %v", err)
	}
	if _, err := db.Dream(context.Background(), ""); err != nil {
		t.Fatalf("dream: %v", err)
	}

	nodes, err := db.ListL1()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("want one node per scene, got %d: %+v", len(nodes), nodes)
	}
	scenes := map[string]bool{first.Scene.SceneID: true, second.Scene.SceneID: true}
	for _, n := range nodes {
		if !isHexID(n.ID) || !isHexID(n.SceneID) {
			t.Fatalf("node ids must cross as hex: %+v", n)
		}
		// EmotionSet is the only thing that tells "never distilled" from a settled (0, 0) — 0 is an extreme
		// reading on both signals.
		if n.EmotionSet {
			t.Fatalf("a node the distillation never named must read as not emotion-set: %+v", n)
		}
		if !scenes[n.SceneID] {
			t.Fatalf("node %s names a scene this domain never opened: %s", n.ID, n.SceneID)
		}
		for _, topic := range n.TopicIDs {
			if !isHexID(topic) {
				t.Fatalf("topic ids must cross as hex: %+v", n)
			}
		}
		for _, edge := range n.EdgeIDs {
			if !isHexID(edge) {
				t.Fatalf("edge ids must cross as hex: %+v", n)
			}
		}
	}
}
