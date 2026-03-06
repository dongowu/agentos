package orchestration

import "testing"

func TestTaskStateMachine_AllowsHappyPathTransitions(t *testing.T) {
	sm := NewTaskStateMachine()

	state := Pending
	var err error

	state, err = sm.Transition(state, Planning)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, err = sm.Transition(state, Queued)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, err = sm.Transition(state, Running)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, err = sm.Transition(state, Evaluating)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = sm.Transition(state, Succeeded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
