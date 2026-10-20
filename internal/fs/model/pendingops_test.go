package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

const (
	pendingOpThree = 3
	pendingOpFour  = 4
	pendingOpFive  = 5
	pendingOpSix   = 6
)

func TestPendingOps(t *testing.T) {
	t.Parallel()
	type testStep struct {
		flushDeps []int // flush these deps
		flushOps  []int // expect these ops

		addOp                  int // add this op
		addDependsOn           []int
		addStoreAs             []int
		addStoreAsAndDependsOn []int
	}

	testCases := []struct {
		name  string
		steps []testStep
	}{
		{
			name: "basic",
			steps: []testStep{
				{addOp: 1, addStoreAs: []int{1}},
				{flushDeps: []int{1}, flushOps: []int{1}},
				{flushDeps: []int{1}, flushOps: []int{}},
			},
		},
		{
			name: "chain at once",
			steps: []testStep{
				{addOp: 1, addStoreAs: []int{1}},
				{addOp: 2, addStoreAs: []int{2}, addDependsOn: []int{1}},
				{addOp: pendingOpThree, addStoreAs: []int{pendingOpThree}, addDependsOn: []int{2}},
				{addOp: pendingOpFour, addStoreAs: []int{pendingOpFour}, addDependsOn: []int{pendingOpThree}},
				{flushDeps: []int{pendingOpFour}, flushOps: []int{1, 2, pendingOpThree, pendingOpFour}},
				{flushDeps: []int{2}, flushOps: []int{}},
				{flushDeps: []int{pendingOpFour}, flushOps: []int{}},
			},
		},
		{
			name: "chain in parts",
			steps: []testStep{
				{addOp: 1, addStoreAs: []int{1}},
				{addOp: 2, addStoreAs: []int{2}, addDependsOn: []int{1}},
				{addOp: pendingOpThree, addStoreAs: []int{pendingOpThree}, addDependsOn: []int{2}},
				{addOp: pendingOpFour, addStoreAs: []int{pendingOpFour}, addDependsOn: []int{pendingOpThree}},
				{flushDeps: []int{2}, flushOps: []int{1, 2}},
				{flushDeps: []int{pendingOpFour}, flushOps: []int{pendingOpThree, pendingOpFour}},
				{flushDeps: []int{2}, flushOps: []int{}},
				{flushDeps: []int{pendingOpFour}, flushOps: []int{}},
			},
		},
		{
			name: "chain at once with more after",
			steps: []testStep{
				{addOp: 1, addStoreAs: []int{1}},
				{addOp: 2, addStoreAs: []int{2}, addDependsOn: []int{1}},
				{addOp: pendingOpThree, addStoreAs: []int{pendingOpThree}, addDependsOn: []int{2}},
				{addOp: pendingOpFour, addStoreAs: []int{pendingOpFour}, addDependsOn: []int{pendingOpThree}},
				{addOp: pendingOpFive, addStoreAs: []int{1}},
				{addOp: pendingOpSix, addStoreAs: []int{1}},
				{flushDeps: []int{pendingOpFour}, flushOps: []int{1, 2, pendingOpThree, pendingOpFour}},
				{flushDeps: []int{2}, flushOps: []int{}},
				{flushDeps: []int{1}, flushOps: []int{pendingOpFive, pendingOpSix}},
			},
		},
		{
			name: "no duplicates in output",
			steps: []testStep{
				{addOp: 1, addStoreAsAndDependsOn: []int{1, 2}},
				{addOp: 2, addStoreAsAndDependsOn: []int{1, 2}},
				{addOp: pendingOpThree, addStoreAsAndDependsOn: []int{1, 2}},
				{addOp: pendingOpFour, addStoreAsAndDependsOn: []int{1, 2}},
				{flushDeps: []int{2}, flushOps: []int{1, 2, pendingOpThree, pendingOpFour}},
				{flushDeps: []int{1}, flushOps: []int{}},
			},
		},
		{
			name: "fast path",
			steps: []testStep{
				{addOp: 1, addDependsOn: []int{1}},
				{addOp: 2, addStoreAs: []int{1}},
				{addOp: pendingOpThree, addDependsOn: []int{1}},
				{addOp: pendingOpFour, addDependsOn: []int{1}, addStoreAs: []int{2}},
				{flushDeps: []int{2}, flushOps: []int{2, pendingOpFour}},
				{flushDeps: []int{1}, flushOps: []int{}},
			},
		},
		{
			name: "double deps",
			steps: []testStep{
				{addOp: 1, addStoreAs: []int{1}, addDependsOn: []int{1}},
				{addOp: 2, addStoreAs: []int{2}, addDependsOn: []int{1, 1}},
				{addOp: pendingOpThree, addStoreAsAndDependsOn: []int{2, pendingOpThree, pendingOpThree}},
				{addOp: pendingOpFour, addStoreAsAndDependsOn: []int{pendingOpThree, pendingOpFour}},
				{flushDeps: []int{pendingOpFour}, flushOps: []int{1, 2, pendingOpThree, pendingOpFour}},
				{flushDeps: []int{1}, flushOps: []int{}},
			},
		},
		{
			name: "tricky release",
			steps: []testStep{
				{addOp: 1, addStoreAsAndDependsOn: []int{1}},
				{addOp: 2, addStoreAsAndDependsOn: []int{1}},
				{addOp: pendingOpThree, addStoreAsAndDependsOn: []int{1, 2}},
				{addOp: pendingOpFour, addStoreAsAndDependsOn: []int{1}},
				{addOp: pendingOpFive, addStoreAsAndDependsOn: []int{1}},
				{flushDeps: []int{2}, flushOps: []int{1, 2, pendingOpThree}},
				{flushDeps: []int{1}, flushOps: []int{pendingOpFour, pendingOpFive}},
			},
		},
		{
			name: "less tricky release",
			steps: []testStep{
				{addOp: 1, addStoreAsAndDependsOn: []int{1}},
				{addOp: pendingOpThree, addStoreAsAndDependsOn: []int{1, 2}},
				{addOp: pendingOpFour, addStoreAsAndDependsOn: []int{1}},
				{addOp: pendingOpFive, addStoreAsAndDependsOn: []int{1}},
				{flushDeps: []int{2}, flushOps: []int{1, pendingOpThree}},
				{flushDeps: []int{1}, flushOps: []int{pendingOpFour, pendingOpFive}},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ops := newPendingOps()

			for i, step := range testCase.steps {
				compareCrashOrder(t, ops)
				if step.addOp != 0 {
					op := ops.addOpInternal()
					op.fsOp = &allocOp{inode: step.addOp}
					for _, dep := range step.addDependsOn {
						ops.addDep(op, depKey{inode: int32(dep)}, depDependsOn)
					}
					for _, dep := range step.addStoreAs {
						ops.addDep(op, depKey{inode: int32(dep)}, depStoreAs)
					}
					for _, dep := range step.addStoreAsAndDependsOn {
						ops.addDep(op, depKey{inode: int32(dep)}, depStoreAsAndDependsOn)
					}
				}

				compareCrashOrder(t, ops)
				if step.flushDeps != nil {
					out := ops.buffer
					for _, dep := range step.flushDeps {
						out = ops.enqueuePrevDeps(ops.lastDepForKey[depKey{inode: int32(dep)}], out)
					}
					out = ops.processQueue(out)

					flushOps := []int{}
					for _, op := range out {
						flushOps = append(flushOps, op.fsOp.(*allocOp).inode)
					}
					ops.releaseBuffer(out)

					if diff := cmp.Diff(step.flushOps, flushOps); diff != "" {
						t.Errorf("step %d: flush deps %v: diff: %s", i+1, step.flushDeps, diff)
					}
				}
			}
		})
	}
}
