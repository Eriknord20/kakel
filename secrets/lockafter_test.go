package secrets

import (
	"bytes"
	"os"
	"regexp"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// The time to lock after is kept in the sealed half: it comes back
// after the vault is opened again, and it is not in the file for
// anybody to read or change without the key.
func TestLockAfterIsSealedIn(t *testing.T) {
	v, key, path := aVault(t)
	if got := v.LockAfter(); got != 0 {
		t.Fatalf("a new vault locks after %v, want never", got)
	}
	if err := v.SetLockAfter(15 * time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// 900 as a value in the JSON, after a colon: the sealed parts are
	// base64, which has no colon, and may hold "900" by chance.
	if bytes.Contains(raw, []byte("lockAfter")) || regexp.MustCompile(`:\s*900\b`).Match(raw) {
		t.Errorf("the time to lock after is in the file in the clear:\n%s", raw)
	}

	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.LockAfter(); got != 0 {
		t.Errorf("a locked vault says it locks after %v", got)
	}
	if err := again.Unlock([]ssh.Signer{key}); err != nil {
		t.Fatal(err)
	}
	if got := again.LockAfter(); got != 15*time.Minute {
		t.Errorf("opened again, it locks after %v, want 15m", got)
	}

	// A change in one window is seen in another, once it looks.
	if err := v.SetLockAfter(0); err != nil {
		t.Fatal(err)
	}
	if _, err := again.Items(); err != nil {
		t.Fatal(err)
	}
	if got := again.LockAfter(); got != 0 {
		t.Errorf("after another window turned it off, it locks after %v", got)
	}
	if err := again.SetLockAfter(-time.Second); err == nil {
		t.Error("a negative time was taken")
	}
}

// Unused counts from the last time a secret was read or the vault
// changed, and listing the names does not reset it.
func TestUnusedCountsFromTheLastUse(t *testing.T) {
	v, _, _ := aVault(t)
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	v.now = func() time.Time { return at }
	it, err := v.Put(Item{Name: "margit"}, "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	at = at.Add(10 * time.Minute)
	if _, err := v.Items(); err != nil {
		t.Fatal(err)
	}
	if got := v.Unused(); got != 10*time.Minute {
		t.Errorf("after listing, unused for %v, want 10m", got)
	}
	if _, err := v.Secret(it.ID); err != nil {
		t.Fatal(err)
	}
	if got := v.Unused(); got != 0 {
		t.Errorf("after reading a secret, unused for %v, want 0", got)
	}
	v.Lock()
	if got := v.Unused(); got != 0 {
		t.Errorf("a locked vault is unused for %v", got)
	}
}
