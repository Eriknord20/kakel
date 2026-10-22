package app

import (
	"slices"
	"testing"

	"github.com/marrasen/kakel/machines"
)

// The machines used are those the pane in front was on, the latest
// first, each once. No pane in front changes nothing.
func TestTheMachinesUsedFollowThePaneInFront(t *testing.T) {
	a := &app{}
	a.st.Panes = []Pane{{ID: "here"}, {ID: "sky", Machine: "sky"}, {ID: "nas", Machine: "nas"}, {ID: "sky2", Machine: "sky"}}
	for _, focus := range []string{"here", "sky", "", "nas", "sky2", "gone"} {
		a.st.Focus = focus
		a.noteMachineUsed()
	}
	if want := []machines.ID{"sky", "nas", ""}; !slices.Equal(a.lastOn, want) {
		t.Fatalf("the machines used are %q, want %q", a.lastOn, want)
	}
}
