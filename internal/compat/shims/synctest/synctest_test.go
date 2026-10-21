//go:build go1.25

package synctest

import "testing"

func TestUnbubbledProbes(t *testing.T) {
	t.Parallel()
	value := 0
	if IsInBubble() || Associate(&value) != Unbubbled || IsAssociated(&value) || Acquire() != nil {
		t.Fatal("invented bubble association")
	}
	Disassociate(&value)
	var bubble *Bubble
	bubble.Release()
	bubble.Run(func() { value++ })
	if value != 1 {
		t.Fatal("nil bubble did not run ordinary callback")
	}
}

func TestBubbleOperationsReject(t *testing.T) {
	t.Parallel()
	for name, f := range map[string]func(){"Run": func() { Run(func() { t.Error("unsupported bubble executed callback") }) }, "Wait": Wait, "non-nil": func() { (&Bubble{}).Run(func() { t.Error("forged bubble") }) }} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("unsupported bubble accepted")
				}
			}()
			f()
		})
	}
}
