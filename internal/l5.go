// L5 big methods of the composition root.

package internal

import (
	"github.com/genosis18m/Long-term-memory-go/internal/plan"
)

// PlanNodeAdd adds one step to the open turn's plan tree and returns its ordinal.
func (db *DB) PlanNodeAdd(agentID uint64, parentSeq uint32, title string) (uint32, error) {
	ac, err := db.lockTurn(agentID)
	if err != nil {
		return 0, err
	}
	defer ac.Mu.Unlock()
	// A new step lands in progress, so its parent cannot fold a summary from it: the rollup would run and
	// change nothing, which is why no RollupTree call follows a create.
	return plan.CreateNode(ac, agentID, plan.NodeSpec{
		TopicID: ac.Turn, ParentSeq: parentSeq, Title: title,
	})
}

// PlanNodeUpdate restates one step of the open turn's tree: its status, and its own Title/Summary,
// where a blank field keeps what the node holds.
func (db *DB) PlanNodeUpdate(agentID uint64, step plan.Step) error {
	ac, err := db.lockTurn(agentID)
	if err != nil {
		return err
	}
	defer ac.Mu.Unlock()
	step.TopicID = ac.Turn
	if err := plan.UpdateNode(ac, agentID, step); err != nil {
		return err
	}
	return plan.RollupTree(ac, agentID, ac.Turn)
}

// PlanState returns the open turn's plan tree as the actual stored statuses — no auto-fold: a parent
// becomes Done only where the host declared it so.
func (db *DB) PlanState(agentID uint64) (*PlanTree, error) {
	ac, err := db.lockTurn(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	return plan.BuildTree(ac, ac.Turn)
}
