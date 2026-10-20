package gosimruntime

import "testing"

func TestGoroutineDiagnosticState(t *testing.T) {
	e := testEngine(t)
	err := e.Run(func() {
		SetProfileLabels("parent")
		done := NewChan[struct{}](0)
		Go(func() {
			if ProfileLabels() != "parent" {
				t.Error("incorrect child diagnostic state")
			}
			SetProfileLabels("child")
			done.Send(struct{}{})
		})
		done.Recv()
		if ProfileLabels() != "parent" {
			t.Error("child modified parent state")
		}
		token, err := e.PrepareMachine(99, 1, "isolated")
		if err != nil {
			t.Error(err)
			return
		}
		e.PublishMachine(token, func() {
			if ProfileLabels() != "" {
				t.Error("diagnostic state leaked between machines")
			}
			done.Send(struct{}{})
		})
		done.Recv()
		e.CancelMachine(token)
		e.DisposeMachine(token)
	})
	if err != nil {
		t.Fatal(err)
	}
}
