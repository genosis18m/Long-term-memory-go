package api

import (
	"context"
	"reflect"
	"testing"
)

// A host wiring these calls to a model writes a tool schema per method: a name, the keys to ask for,
// and what the answer is good for.
type toolRow struct {
	method   string
	tool     string
	argc     int            // arguments the method really takes (receiver and context aside)
	keyTypes []reflect.Type // shapes whose json keys the row must list
	keyNames []string       // other names the row must list
}

var toolRows = []toolRow{
	{"Search", "memory_search", 1, []reflect.Type{reflect.TypeOf(SearchQuery{})}, nil},
	{"AppendArchive", "memory_record", 1, []reflect.Type{reflect.TypeOf(ArchiveInput{})}, nil},
	{"Update", "memory_close_turn", 1, []reflect.Type{reflect.TypeOf(TurnEnd{})}, nil},
	{"Dream", "memory_dream", 1, nil, []string{"scene_id"}},
	{"GetL0", "memory_profile_get", 0, nil, nil},
	{"UpdateL0", "memory_profile_update", 1, []reflect.Type{reflect.TypeOf(ProfileInput{})}, nil},
	{"ListL1", "memory_associations", 0, nil, nil},
	{"ListScenes", "memory_scenes", 1, nil, []string{"l3_id"}},
	{"SceneContext", "memory_scene_read", 1, nil, []string{"scene_id"}},
	{"GetL3", "memory_graph_get", 1, nil, []string{"id"}},
	{"ListL3", "memory_graph_list", 0, nil, nil},
	{"ImportL3", "memory_graph_import", 2,
		[]reflect.Type{reflect.TypeOf(L3ImportItem{}), reflect.TypeOf(L3Relation{})},
		[]string{"items", "mode"}},
	{"QueryL3Nodes", "memory_nodes_query", 1, []reflect.Type{reflect.TypeOf(L3NodeQuery{})}, nil},
	{"QueryL3Subgraph", "memory_subgraph", 4, nil,
		[]string{"graph_id", "start_node_id", "max_depth", "edge_kinds"}},
	{"SearchL4", "memory_archive_search", 1, []reflect.Type{reflect.TypeOf(L4Query{})}, nil},
	{"PlanNodeAdd", "plan_add_step", 2, nil, []string{"parent_seq", "title"}},
	{"PlanNodeUpdate", "plan_update_step", 1, []reflect.Type{reflect.TypeOf(PlanStep{})}, nil},
	{"PlanState", "plan_state", 0, nil, nil},
}

// adminFace is the other half of the session surface; none of it belongs in the tool table.
var adminFace = []string{
	"UpdateScene", "RenameTopic", "MergeScenes", "DeleteScene", "DeleteTopic",
	"UpdateL3", "DeleteL3", "AgentID",
}

// TestEveryTaskFaceMethodHasAToolRow closes the other direction: a method added to the session surface
// is only absent from the tool table on purpose.
func TestEveryTaskFaceMethodHasAToolRow(t *testing.T) {
	listed := map[string]bool{}
	for _, row := range toolRows {
		listed[row.method] = true
	}
	admin := map[string]bool{}
	for _, method := range adminFace {
		admin[method] = true
	}
	handle := reflect.TypeOf(&Session{})
	for i := 0; i < handle.NumMethod(); i++ {
		method := handle.Method(i)
		if !method.IsExported() || admin[method.Name] {
			continue
		}
		if !listed[method.Name] {
			t.Errorf("Session.%s is exported and not admin-face, so the tool table owes it a row", method.Name)
		}
	}
}

// TestToolRowsFollowTheMethodSet checks the tool table against the code: every listed method exists
// and takes what the row says it takes.
func TestToolRowsFollowTheMethodSet(t *testing.T) {
	handle := reflect.TypeOf(&Session{})
	for _, row := range toolRows {
		method, ok := handle.MethodByName(row.method)
		if !ok {
			t.Fatalf("Session.%s is gone, so the tool table names a call that no longer exists", row.method)
		}
		if got := len(methodArgs(method.Type)); got != row.argc {
			t.Errorf("Session.%s takes %d arguments, the table describes %d", row.method, got, row.argc)
		}
	}
}

func TestTaskFaceCountMatchesToolTable(t *testing.T) {
	admin := map[string]bool{}
	for _, m := range adminFace {
		admin[m] = true
	}
	handle := reflect.TypeOf(&Session{})
	taskCount := 0
	for i := 0; i < handle.NumMethod(); i++ {
		method := handle.Method(i)
		if method.IsExported() && !admin[method.Name] {
			taskCount++
		}
	}
	if taskCount != len(toolRows) {
		t.Fatalf("the task face holds %d methods but the tool table describes %d", taskCount, len(toolRows))
	}
}

func methodArgs(fn reflect.Type) []reflect.Type {
	var out []reflect.Type
	ctx := reflect.TypeOf((*context.Context)(nil)).Elem()
	for i := 1; i < fn.NumIn(); i++ {
		if fn.In(i) == ctx {
			continue
		}
		out = append(out, fn.In(i))
	}
	return out
}
