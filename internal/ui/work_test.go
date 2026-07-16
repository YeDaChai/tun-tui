package ui

import "testing"

func TestWorkState(t *testing.T) {
	if workIdle.busy() || workIdle.spinning() {
		t.Fatal("idle should be neither busy nor spinning")
	}
	for _, s := range []workState{workConnecting, workLoadingNodes, workTesting, workActing} {
		if !s.busy() {
			t.Fatalf("%v should be busy", s)
		}
	}
	if !workConnecting.spinning() || !workLoadingNodes.spinning() {
		t.Fatal("connect/load should spin")
	}
	if workTesting.spinning() || workActing.spinning() {
		t.Fatal("test/act should not spin")
	}
}
