package remote

import "testing"

func TestAWindowThroughARelayIsSavedByItsCode(t *testing.T) {
	h := Host{Name: "desk", Address: "https://relay.example.com/k/abc", Window: true, Code: "abcd efgh"}
	if err := h.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := h.ServeAddr(); got != "https://relay.example.com/k/abc#ABCD-EFGH" {
		t.Errorf("ServeAddr = %q", got)
	}
	if got := h.Target(); got != "ABCD-EFGH on relay.example.com" {
		t.Errorf("Target = %q, which should not show the secret", got)
	}
	for _, bad := range []Host{
		{Name: "a", Address: "relay.example.com", Window: true, Code: "ABCD-EFGH"},
		{Name: "b", Address: "https://relay.example.com/k/abc", Window: true, Code: "nope"},
		{Name: "c", Address: "https://relay.example.com/k/abc", Code: "ABCD-EFGH"},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("%s was taken", bad.Name)
		}
	}
}
