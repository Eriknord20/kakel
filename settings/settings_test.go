package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// at is a settings file in a directory the test owns.
func at(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "settings.json")
}

// What is saved comes back on the next run.
func TestWhatIsSavedComesBack(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if err := s.PutServe(2300, ReachAnywhere); err != nil {
		t.Fatalf("save: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	port, have := again.ServePort()
	if !have || port != 2300 {
		t.Errorf("the port came back as %d, %v; want 2300", port, have)
	}
	reach, have := again.ServeReach()
	if !have || reach != ReachAnywhere {
		t.Errorf("the reach came back as %q, %v; want %q", reach, have, ReachAnywhere)
	}
}

// Port 0 is a choice, not an empty one: it means whichever port is free,
// and it has to survive being saved.
func TestPortZeroIsRemembered(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if err := s.PutServe(0, ReachHere); err != nil {
		t.Fatalf("save: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	if port, have := again.ServePort(); !have || port != 0 {
		t.Errorf("the port came back as %d, %v; want 0 saved", port, have)
	}
}

// The file says which version wrote it, so a later shape can be told
// from this one.
func TestTheFileSaysItsVersion(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := s.PutServe(2300, ReachHere); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read it back: %v", err)
	}
	if !strings.Contains(string(raw), `"version": 1`) {
		t.Errorf("the file has no version in it:\n%s", raw)
	}
}

// A file that is not there is what the first run looks like: nothing is
// remembered and nothing is wrong.
func TestAMissingFileIsNotAnError(t *testing.T) {
	s, err := Load(at(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if port, have := s.ServePort(); have {
		t.Errorf("a missing file remembered port %d", port)
	}
	if reach, have := s.ServeReach(); have {
		t.Errorf("a missing file remembered the reach %q", reach)
	}
	if err := s.Err(); err != nil {
		t.Errorf("a missing file made the settings unsaveable: %v", err)
	}
}

// Settings that could not be read are never written over. The file is
// somebody's, and an empty one in its place loses it for good.
func TestUnreadableSettingsAreNotWrittenOver(t *testing.T) {
	broken := []struct{ what, body string }{
		{"not JSON at all", "{"},
		{"no version", `{"servePort": 2300}`},
		{"a newer version", `{"version": 99, "servePort": 2300}`},
		{"a field this build does not know, within a setting", `{"version": 1, "favourites": [{"path": "/x", "whatIsThis": 14}]}`},
		{"a port that is not one", `{"version": 1, "servePort": 70000}`},
		{"a reach that means nothing", `{"version": 1, "serveReach": "everywhere"}`},
		{"an agent host with no name", `{"version": 1, "agentHost": ""}`},
		{"a shell with no id", `{"version": 1, "shell": ""}`},
		{"more than one set of settings", `{"version": 1}{"version": 1}`},
		{"a key written twice", `{"version": 1, "servePort": 2300, "servePort": 9000}`},
	}
	for _, b := range broken {
		t.Run(b.what, func(t *testing.T) {
			path := at(t)
			if err := os.WriteFile(path, []byte(b.body), 0o600); err != nil {
				t.Fatalf("write the file: %v", err)
			}
			was, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read the file: %v", err)
			}

			s, loadErr := Load(path)
			if loadErr == nil {
				t.Fatalf("%s loaded clean", b.what)
			}
			if s.Err() == nil {
				t.Error("the settings do not say they cannot be saved")
			}

			err = s.PutServe(2300, ReachHere)
			if !errors.Is(err, ErrUnsaveable) {
				t.Errorf("saving over it gave %v, want ErrUnsaveable", err)
			}
			now, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read the file again: %v", err)
			}
			if !bytes.Equal(was, now) {
				t.Errorf("the file was written over:\n%s\nwant\n%s", now, was)
			}
		})
	}
}

// A file that went bad after it was read is not written over either. The
// file is read again before every change, so a window that has been open
// for a week does not save from a week-old copy.
func TestAFileThatWentBadAfterItWasReadIsNotWrittenOver(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	err = s.PutServe(2300, ReachHere)

	if !errors.Is(err, ErrUnsaveable) {
		t.Errorf("saving gave %v, want ErrUnsaveable", err)
	}
	now, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file again: %v", err)
	}
	if string(now) != "{" {
		t.Errorf("the file was written over:\n%s", now)
	}
}

// Settings with nowhere to live say so and refuse to save.
func TestSettingsWithNowhereToLiveRefuseToSave(t *testing.T) {
	s := Unusable(errors.New("no configuration directory"))

	if s.Err() == nil {
		t.Error("they do not say why they are unusable")
	}
	if err := s.PutServe(2300, ReachHere); !errors.Is(err, ErrUnsaveable) {
		t.Errorf("saving gave %v, want ErrUnsaveable", err)
	}
}

// A write that fails leaves the settings that were there, and leaves no
// half-written file behind.
func TestAFailedWriteLeavesTheOldFile(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := s.PutServe(2300, ReachHere); err != nil {
		t.Fatalf("first save: %v", err)
	}
	was, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file: %v", err)
	}

	// The disk gives up on the last step, after the new file is written
	// and before it takes the old one's place.
	wasRename := rename
	rename = func(string, string) error { return errors.New("the disk gave up") }
	t.Cleanup(func() { rename = wasRename })

	err = s.PutServe(9000, ReachAnywhere)

	if err == nil {
		t.Fatal("a failed write was reported as a save")
	}
	now, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file again: %v", err)
	}
	if !bytes.Equal(was, now) {
		t.Errorf("the old settings are gone:\n%s\nwant\n%s", now, was)
	}
	left, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read the directory: %v", err)
	}
	if len(left) != 1 {
		names := make([]string, 0, len(left))
		for _, e := range left {
			names = append(names, e.Name())
		}
		t.Errorf("the directory holds %v, want only the settings", names)
	}
	// And the settings still hold what is on disk, not what failed to
	// get there.
	if port, _ := s.ServePort(); port != 2300 {
		t.Errorf("the settings hold port %d, want the 2300 that is saved", port)
	}
}

// The one key written twice is named, so the user knows what to repair.
//
// Go's decoder keeps the last of a repeated key, so a file asking for
// two ports would quietly become whichever came second -- and the next
// save would make that permanent.
func TestAKeyWrittenTwiceIsNamed(t *testing.T) {
	path := at(t)
	const twice = `{"version": 1, "servePort": 2300, "servePort": 9000}`
	if err := os.WriteFile(path, []byte(twice), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("a file asking for two ports loaded as one")
	}
	if !strings.Contains(err.Error(), "servePort") || !strings.Contains(err.Error(), "twice") {
		t.Errorf("it said %v, without naming what is in it twice", err)
	}
}

// Settings from a newer kakel say so, even when that kakel also
// added a field this build does not know.
//
// The strict decode used to run first, so the user was told "json:
// unknown field" about a file whose real trouble is that it belongs to a
// later version.
func TestNewerSettingsSaySoRatherThanNamingTheirNewField(t *testing.T) {
	path := at(t)
	const newer = `{"version": 2, "servePort": 2300, "whatIsThis": 14}`
	if err := os.WriteFile(path, []byte(newer), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("settings from a newer kakel loaded clean")
	}
	if !strings.Contains(err.Error(), "newer kakel") {
		t.Errorf("it said %v, without saying the file is from a newer kakel", err)
	}
}

// A settings file linked in from somewhere else goes on being the file
// that is written, rather than being quietly replaced by a copy.
func TestSettingsSaveThroughASymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.json")
	if err := os.WriteFile(real, []byte(`{"version": 1}`), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}
	link := filepath.Join(dir, "settings.json")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	s, err := Load(link)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if err := s.PutServe(2300, ReachHere); err != nil {
		t.Fatalf("save: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the link was replaced by a file, so the real one stopped being updated")
	}
	raw, err := os.ReadFile(real)
	if err != nil {
		t.Fatalf("read the real file: %v", err)
	}
	if !strings.Contains(string(raw), "2300") {
		t.Errorf("the real file was not updated:\n%s", raw)
	}
}

// Two goroutines saving at once both get an answer, and what is on disk
// is one of the two rather than a mix.
func TestSavingFromTwoGoroutines(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	done := make(chan error, 2)
	for _, port := range []int{2300, 9000} {
		go func() { done <- s.PutServe(port, ReachHere) }()
	}
	for range 2 {
		if err := <-done; err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	port, saved := again.ServePort()
	if !saved || (port != 2300 && port != 9000) {
		t.Errorf("the file holds port %d, %v; want one of the two that were saved", port, saved)
	}
}

// Which agent the hand-over dialog was set to comes back on the next
// run.
func TestTheAgentHostIsRemembered(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if name, have := s.AgentHost(); have {
		t.Errorf("a missing file remembered the agent %q", name)
	}

	if err := s.PutAgentHost("Codex"); err != nil {
		t.Fatalf("save: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	name, have := again.AgentHost()
	if !have || name != "Codex" {
		t.Errorf("the agent came back as %q, %v; want Codex", name, have)
	}
}

// Remembering the agent leaves what else was saved alone, because it
// reads the file before it writes it.
func TestRememberingTheAgentKeepsWhatElseWasSaved(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := s.PutServe(2300, ReachAnywhere); err != nil {
		t.Fatalf("save the port: %v", err)
	}

	if err := s.PutAgentHost("Cursor"); err != nil {
		t.Fatalf("save the agent: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	if port, have := again.ServePort(); !have || port != 2300 {
		t.Errorf("the port came back as %d, %v; want 2300", port, have)
	}
	if name, have := again.AgentHost(); !have || name != "Cursor" {
		t.Errorf("the agent came back as %q, %v; want Cursor", name, have)
	}
}

// Settings that could not be read are not written over by remembering an
// agent either.
func TestTheAgentIsNotSavedOverUnreadableSettings(t *testing.T) {
	path := at(t)
	const broken = "{"
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}
	s, loadErr := Load(path)
	if loadErr == nil {
		t.Fatal("a broken file loaded clean")
	}

	err := s.PutAgentHost("Codex")

	if !errors.Is(err, ErrUnsaveable) {
		t.Errorf("saving gave %v, want ErrUnsaveable", err)
	}
	now, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file again: %v", err)
	}
	if string(now) != broken {
		t.Errorf("the file was written over:\n%s", now)
	}
}

// An agent with no name is not a choice, and is refused rather than
// written down.
func TestAnAgentWithNoNameIsRefused(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if err := s.PutAgentHost(""); err == nil {
		t.Error("an agent with no name was saved")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		raw, _ := os.ReadFile(path)
		t.Errorf("a file was written anyway:\n%s", raw)
	}
}

// Which shell a new pane runs comes back on the next run.
func TestTheShellIsRemembered(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if id, have := s.Shell(); have {
		t.Errorf("a missing file remembered the shell %q", id)
	}

	if err := s.PutShell("wsl:Ubuntu"); err != nil {
		t.Fatalf("save: %v", err)
	}

	if id, have := s.Shell(); !have || id != "wsl:Ubuntu" {
		t.Errorf("the shell reads back as %q, %v; want wsl:Ubuntu", id, have)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	id, have := again.Shell()
	if !have || id != "wsl:Ubuntu" {
		t.Errorf("the shell came back as %q, %v; want wsl:Ubuntu", id, have)
	}
}

// Remembering the shell leaves what else the same settings saved alone.
// Another window's work is TestRememberingTheShellKeepsASecondWindowsWork.
func TestRememberingTheShellKeepsWhatElseWasSaved(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := s.PutServe(2300, ReachAnywhere); err != nil {
		t.Fatalf("save the port: %v", err)
	}
	if err := s.PutAgentHost("Codex"); err != nil {
		t.Fatalf("save the agent: %v", err)
	}

	if err := s.PutShell("pwsh"); err != nil {
		t.Fatalf("save the shell: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	if port, have := again.ServePort(); !have || port != 2300 {
		t.Errorf("the port came back as %d, %v; want 2300", port, have)
	}
	if reach, have := again.ServeReach(); !have || reach != ReachAnywhere {
		t.Errorf("the reach came back as %q, %v; want %q", reach, have, ReachAnywhere)
	}
	if name, have := again.AgentHost(); !have || name != "Codex" {
		t.Errorf("the agent came back as %q, %v; want Codex", name, have)
	}
	if id, have := again.Shell(); !have || id != "pwsh" {
		t.Errorf("the shell came back as %q, %v; want pwsh", id, have)
	}
}

// Remembering the shell does not write over a second window's work, because
// it reads the file again first.
func TestRememberingTheShellKeepsASecondWindowsWork(t *testing.T) {
	path := at(t)
	one, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	two, err := Load(path)
	if err != nil {
		t.Fatalf("load a second window: %v", err)
	}
	if err := two.PutServe(9000, ReachAnywhere); err != nil {
		t.Fatalf("the second window saves the port: %v", err)
	}

	if err := one.PutShell("cmd"); err != nil {
		t.Fatalf("save the shell: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	if port, have := again.ServePort(); !have || port != 9000 {
		t.Errorf("the port came back as %d, %v; want the 9000 the second window saved", port, have)
	}
	if id, have := again.Shell(); !have || id != "cmd" {
		t.Errorf("the shell came back as %q, %v; want cmd", id, have)
	}
}

// A shell with no id is not a choice, and is refused rather than written
// down.
func TestAShellWithNoIdIsRefused(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if err := s.PutShell(""); err == nil {
		t.Error("a shell with no id was saved")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		raw, _ := os.ReadFile(path)
		t.Errorf("a file was written anyway:\n%s", raw)
	}
}

// A file naming a shell with no id is turned away in words the user can
// act on.
func TestAFileWithAnEmptyShellSaysWhatIsWrong(t *testing.T) {
	path := at(t)
	if err := os.WriteFile(path, []byte(`{"version": 1, "shell": ""}`), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("a shell with no id loaded clean")
	}
	if !strings.Contains(err.Error(), "shell") {
		t.Errorf("it said %v, without saying the shell is what is wrong", err)
	}
}

// Settings that could not be read are not written over by remembering a
// shell either.
func TestTheShellIsNotSavedOverUnreadableSettings(t *testing.T) {
	path := at(t)
	const broken = "{"
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}
	s, loadErr := Load(path)
	if loadErr == nil {
		t.Fatal("a broken file loaded clean")
	}

	err := s.PutShell("pwsh")

	if !errors.Is(err, ErrUnsaveable) {
		t.Errorf("saving gave %v, want ErrUnsaveable", err)
	}
	now, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file again: %v", err)
	}
	if string(now) != broken {
		t.Errorf("the file was written over:\n%s", now)
	}
}

// A write that fails leaves the shell that was saved, on disk and in the
// settings.
func TestAFailedWriteLeavesTheShellThatWasSaved(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := s.PutShell("cmd"); err != nil {
		t.Fatalf("first save: %v", err)
	}
	was, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file: %v", err)
	}

	// The disk gives up on the last step, after the new file is written
	// and before it takes the old one's place.
	wasRename := rename
	rename = func(string, string) error { return errors.New("the disk gave up") }
	t.Cleanup(func() { rename = wasRename })

	err = s.PutShell("pwsh")

	if err == nil {
		t.Fatal("a failed write was reported as a save")
	}
	now, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file again: %v", err)
	}
	if !bytes.Equal(was, now) {
		t.Errorf("the old settings are gone:\n%s\nwant\n%s", now, was)
	}
	if id, _ := s.Shell(); id != "cmd" {
		t.Errorf("the settings hold the shell %q, want the cmd that is saved", id)
	}
}

// Two goroutines remembering different things both survive, which is
// what the reread before every save is for.
func TestSavingDifferentThingsAtOnce(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	done := make(chan error, 2)
	go func() { done <- s.PutShell("pwsh") }()
	go func() { done <- s.PutAgentHost("Codex") }()
	for range 2 {
		if err := <-done; err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	if id, have := again.Shell(); !have || id != "pwsh" {
		t.Errorf("the shell came back as %q, %v; want pwsh", id, have)
	}
	if name, have := again.AgentHost(); !have || name != "Codex" {
		t.Errorf("the agent came back as %q, %v; want Codex", name, have)
	}
}

// Settings that cannot be read refuse to remember a shell.
func TestTheShellIsNotSavedOverUnusableSettings(t *testing.T) {
	s := Unusable(errors.New("no configuration directory"))

	if err := s.PutShell("cmd"); !errors.Is(err, ErrUnsaveable) {
		t.Errorf("saving gave %v, want ErrUnsaveable", err)
	}
}

// A saved command with nothing to run names nothing, so the file is
// turned away rather than read back as a command that cannot be offered.
func TestASavedCommandWithNothingToRunIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	raw := `{"version":1,"commands":[{"line":"make deploy"},{"line":""}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("a command with nothing to run was read back")
	}
	// The one that is wrong, by the place it sits in the file: a message
	// that does not say which cannot be acted on.
	if !strings.Contains(err.Error(), "saved command 2") {
		t.Errorf("it said %q, want it to name the second command", err)
	}
}

// The same line twice is turned away: the dialog steps through the
// commands, and stepping from one to its twin lands on itself.
func TestASavedCommandTwiceOverIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	raw := `{"version":1,"commands":[{"line":"make deploy"},{"line":"make deploy"}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("the same command twice was read back")
	}
	if !strings.Contains(err.Error(), "twice") {
		t.Errorf("it said %q", err)
	}
}

// And a file whose commands are all good is read back whole.
func TestSavedCommandsAreReadBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	raw := `{"version":1,"commands":[` +
		`{"line":"make deploy","dir":"/src","host":"margit"},{"line":"ls -la"}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	set, err := Load(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := set.Commands()
	want := []SavedCommand{
		{Line: "make deploy", Dir: "/src", Host: "margit"},
		{Line: "ls -la"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("it read %v, want %v", got, want)
	}
}

// What Commands gives back is the caller's own, so changing it cannot
// change what is saved.
func TestCommandsGivesBackACopy(t *testing.T) {
	set := settingsAt(t)
	if err := set.KeepCommand(SavedCommand{Line: "make deploy"}, 10); err != nil {
		t.Fatalf("keep: %v", err)
	}

	got := set.Commands()
	got[0].Line = "rm -rf /"

	if again := set.Commands(); again[0].Line != "make deploy" {
		t.Errorf("the settings now say %q", again[0].Line)
	}
}

// Keeping a command puts it at the front, and keeping one already there
// moves it rather than doubling it.
func TestKeepCommandPutsItAtTheFront(t *testing.T) {
	set := settingsAt(t)
	for _, line := range []string{"first", "second", "third"} {
		if err := set.KeepCommand(SavedCommand{Line: line, Dir: "/old"}, 10); err != nil {
			t.Fatalf("keep %q: %v", line, err)
		}
	}

	if err := set.KeepCommand(SavedCommand{Line: "first", Dir: "/new"}, 10); err != nil {
		t.Fatalf("keep it again: %v", err)
	}

	got := set.Commands()
	want := []SavedCommand{
		{Line: "first", Dir: "/new"},
		{Line: "third", Dir: "/old"},
		{Line: "second", Dir: "/old"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("it holds %v, want %v", got, want)
	}
}

// The list is capped, and it is the one kept longest ago that goes.
func TestKeepCommandDropsTheOldest(t *testing.T) {
	set := settingsAt(t)
	for i := range 6 {
		if err := set.KeepCommand(SavedCommand{Line: "echo " + strconv.Itoa(i)}, 4); err != nil {
			t.Fatalf("keep %d: %v", i, err)
		}
	}

	var got []string
	for _, cmd := range set.Commands() {
		got = append(got, cmd.Line)
	}
	want := []string{"echo 5", "echo 4", "echo 3", "echo 2"}
	if !slices.Equal(got, want) {
		t.Errorf("it holds %v, want %v", got, want)
	}
}

// DropCommand takes one out and leaves the rest in order.
func TestDropCommandTakesOneOut(t *testing.T) {
	set := settingsAt(t)
	for _, line := range []string{"first", "second", "third"} {
		if err := set.KeepCommand(SavedCommand{Line: line}, 10); err != nil {
			t.Fatalf("keep %q: %v", line, err)
		}
	}

	if err := set.DropCommand("second"); err != nil {
		t.Fatalf("drop: %v", err)
	}

	var got []string
	for _, cmd := range set.Commands() {
		got = append(got, cmd.Line)
	}
	if want := []string{"third", "first"}; !slices.Equal(got, want) {
		t.Errorf("it holds %v, want %v", got, want)
	}
}

// A command another window saved while this one was not looking is
// still there afterwards. The list is edited after the file is reread,
// not before.
func TestKeepCommandDoesNotLoseWhatAnotherWindowSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	one, err := Load(path)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	two, err := Load(path)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	// Read by both before either writes, which is what makes one of
	// them stale.
	_, _ = one.Commands(), two.Commands()

	if err := one.KeepCommand(SavedCommand{Line: "make deploy"}, 10); err != nil {
		t.Fatalf("the first window: %v", err)
	}
	if err := two.KeepCommand(SavedCommand{Line: "ls -la"}, 10); err != nil {
		t.Fatalf("the second window: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("read it again: %v", err)
	}
	var got []string
	for _, cmd := range again.Commands() {
		got = append(got, cmd.Line)
	}
	if want := []string{"ls -la", "make deploy"}; !slices.Equal(got, want) {
		t.Errorf("the file holds %v, want both windows' commands", got)
	}
}

// settingsAt is settings in a directory the test owns.
func settingsAt(t *testing.T) *Settings {
	t.Helper()
	set, err := Load(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	return set
}

// Whether the window was serving is remembered, so the next one can
// offer to serve again.
func TestWhetherItWasServingIsRemembered(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.ServeOn() {
		t.Error("a missing file says the window was serving")
	}

	if err := s.PutServeOn(true); err != nil {
		t.Fatalf("save: %v", err)
	}

	if !s.ServeOn() {
		t.Error("it does not read back")
	}
	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	if !again.ServeOn() {
		t.Error("the next window does not know it was serving")
	}
	// And it can be taken back.
	if err := again.PutServeOn(false); err != nil {
		t.Fatalf("save it off: %v", err)
	}
	third, err := Load(path)
	if err != nil {
		t.Fatalf("load a third time: %v", err)
	}
	if third.ServeOn() {
		t.Error("it still says the window was serving")
	}
}

// Writing it down keeps what a second window saved in between.
func TestRememberingItWasServingKeepsASecondWindowsWork(t *testing.T) {
	path := at(t)
	one, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	two, err := Load(path)
	if err != nil {
		t.Fatalf("load a second window: %v", err)
	}
	if err := two.PutServe(9000, ReachAnywhere); err != nil {
		t.Fatalf("the second window saves the port: %v", err)
	}

	if err := one.PutServeOn(true); err != nil {
		t.Fatalf("save: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	if port, have := again.ServePort(); !have || port != 9000 {
		t.Errorf("the port came back as %d, %v; want the 9000 the second window saved", port, have)
	}
	if !again.ServeOn() {
		t.Error("it does not say the window was serving")
	}
}

// It is not saved over settings that could not be read, and the value
// in hand is left as it was.
func TestItIsNotSavedOverUnreadableSettings(t *testing.T) {
	path := at(t)
	const broken = "{"
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}
	s, loadErr := Load(path)
	if loadErr == nil {
		t.Fatal("a broken file loaded clean")
	}

	err := s.PutServeOn(true)

	if !errors.Is(err, ErrUnsaveable) {
		t.Errorf("saving gave %v, want ErrUnsaveable", err)
	}
	if s.ServeOn() {
		t.Error("the value in hand was changed by a save that failed")
	}
	now, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file again: %v", err)
	}
	if string(now) != broken {
		t.Errorf("the file is now %q, want the %q it was", now, broken)
	}
}

// A settings file holding a copy with nothing to copy is refused, the
// way one holding a command with nothing to run is.
func TestACopyWithNothingToCopyIsRefused(t *testing.T) {
	path := at(t)
	const body = `{"version": 1, "copies": [{"at": "/a", "into": "/b", "names": []}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("a copy with nothing to copy loaded")
	}
}

// The same copy twice over is refused: forgetting either would take both
// out of the file and leave a row nothing answers.
func TestTheSameCopyTwiceIsRefused(t *testing.T) {
	path := at(t)
	const one = `{"at": "/a", "into": "/b", "names": ["x"]}`
	if err := os.WriteFile(path, []byte(`{"version": 1, "copies": [`+one+`, `+one+`]}`), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("the same copy twice loaded")
	}
}

// A setting a newer kakel wrote, which this one doesn't know, as one
// machine's newer kakel does to a roaming profile another shares, is
// kept: the settings load, save, and write it back as it was.
func TestASettingFromANewerKakelIsKept(t *testing.T) {
	path := at(t)
	body := `{"version": 1, "servePort": 2300, "whatIsThis": {"a": [1, 2]}, "andThis": "x"}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutServe(2400, ReachHere); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("written, the file doesn't read: %v\n%s", err, raw)
	}
	if back["andThis"] != "x" || back["servePort"] != float64(2400) {
		t.Fatalf("written back:\n%s", raw)
	}
	if w, ok := back["whatIsThis"].(map[string]any); !ok || len(w["a"].([]any)) != 2 {
		t.Fatalf("the unknown setting came back as %v", back["whatIsThis"])
	}
	// And read again, it is still kept.
	s, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutServe(2500, ReachHere); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if !strings.Contains(string(raw), `"andThis": "x"`) {
		t.Fatalf("saved twice:\n%s", raw)
	}
}

// The default favourites go in once, after those there are, leaving out
// a folder already there; removed, they stay removed.
func TestDefaultFavouritesGoInOnce(t *testing.T) {
	s, err := Load(at(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutFavourites([]Favourite{{Path: "/mine"}, {Path: "/home/me/Desktop"}}); err != nil {
		t.Fatal(err)
	}
	defaults := []Favourite{{Path: "/home/me/Desktop", Icon: "monitor", Color: "teal"}, {Path: "/home/me/Downloads", Icon: "download", Color: "green"}}
	if err := s.SeedFavourites(defaults); err != nil {
		t.Fatal(err)
	}
	favs, _ := s.Favourites()
	if len(favs) != 3 || favs[0].Path != "/mine" || favs[2].Path != "/home/me/Downloads" || favs[2].Icon != "download" {
		t.Fatalf("seeded, the favourites are %+v", favs)
	}
	if err := s.PutFavourites(favs[:1]); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedFavourites(defaults); err != nil {
		t.Fatal(err)
	}
	if favs, _ := s.Favourites(); len(favs) != 1 {
		t.Fatalf("seeded again, the removed came back: %+v", favs)
	}
}

// The alerts and the title bar are kept, and read back; a user who has
// not chosen gets the defaults.
func TestTheAlertsAndTheTitleBarAreKept(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), File))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if s.Sounds() != DefaultSounds || s.Rings() != DefaultRings || s.SystemTitleBar() {
		t.Fatalf("unchosen, sounds %+v, rings %+v, system title bar %v", s.Sounds(), s.Rings(), s.SystemTitleBar())
	}
	sounds := Alerts{Interface: true, Lost: true}
	rings := Alerts{Interface: true, Finished: true}
	if err := s.PutLook(sounds, rings, true); err != nil {
		t.Fatal(err)
	}
	again, err := Load(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	want := rings
	want.Interface = false
	if again.Sounds() != sounds || again.Rings() != want || !again.SystemTitleBar() {
		t.Fatalf("read back, sounds %+v, rings %+v, system title bar %v", again.Sounds(), again.Rings(), again.SystemTitleBar())
	}
}

// The paste check is on until it is turned off, and stays off.
func TestThePasteCheckIsOnUntilTurnedOff(t *testing.T) {
	path := at(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !s.PasteCheck() {
		t.Error("with nothing saved, the paste check is off")
	}
	if err := s.PutPasteCheck(false); err != nil {
		t.Fatalf("save: %v", err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	if again.PasteCheck() {
		t.Error("turned off, the paste check came back on")
	}
}

// The file manager window's place is kept apart from the window's, and
// read back after a load.
func TestTheFileManagerWindowsPlaceIsKeptApart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutFilesWindow(WindowPlace{X: 10, Y: 20, W: 900, H: 600, Maximized: true}); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := again.FilesWindow(); !ok || p != (WindowPlace{X: 10, Y: 20, W: 900, H: 600, Maximized: true}) {
		t.Fatalf("read back %v (%v)", p, ok)
	}
	if _, ok := again.Window(); ok {
		t.Fatal("the file manager window's place was kept as the window's")
	}
}

func TestTheRelayIsRemembered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Relay{URL: "https://relay.example.com/k/abc", On: true, NoPort: true, Via: "s1"}
	if err := s.PutRelay(want); err != nil {
		t.Fatal(err)
	}
	if err := s.PutRelayLast("https://other.example.com/k/def"); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Relay(); got != want {
		t.Errorf("came back as %+v", got)
	}
	if again.TunnelsConnect() {
		t.Error("tunnels may connect, never set")
	}
	if err := again.PutTunnelsConnect(true); err != nil {
		t.Fatal(err)
	}
	if third, err := Load(path); err != nil || !third.TunnelsConnect() {
		t.Errorf("tunnels may connect came back off, %v", err)
	}
	if got := again.RelayLast(); got != "https://other.example.com/k/def" {
		t.Errorf("the last relay came back as %q", got)
	}
}

// A profile is fitted to the profiles the file holds as it is saved,
// not to the ones this window last read. So two kakels that each add
// one under the same name keep both, rather than the second writing
// over the first, and a save the fit refuses leaves the file alone.
func TestAProfileIsFittedToTheFileAsItIsSaved(t *testing.T) {
	path := at(t)
	here, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	there, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	fresh := func(have []Profile) (Profile, error) {
		p := Profile{Name: "MSYS2", Command: []string{"bash"}, ID: "profile:msys2"}
		for n := 2; slices.ContainsFunc(have, func(at Profile) bool { return at.ID == p.ID }); n++ {
			p.ID = "profile:msys2-" + strconv.Itoa(n)
		}
		return p, nil
	}
	if err := there.PutProfile(Profile{}, fresh); err != nil {
		t.Fatal(err)
	}
	if err := here.PutProfile(Profile{}, fresh); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("command line already used")
	if err := here.PutProfile(Profile{ID: "profile:zsh", Name: "Zsh", Command: []string{"zsh"}}, func([]Profile) (Profile, error) {
		return Profile{}, refused
	}); !errors.Is(err, refused) {
		t.Fatalf("a refused profile was saved, %v", err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range again.Profiles() {
		got = append(got, p.ID)
	}
	if want := []string{"profile:msys2", "profile:msys2-2"}; !slices.Equal(got, want) {
		t.Fatalf("the file holds %q, want %q", got, want)
	}
}

// A profile is added after the rest, put in place of the one with its
// id when edited, and taken out alone; what is saved comes back.
func TestProfilesAreAddedEditedAndRemoved(t *testing.T) {
	path := at(t)
	set, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	bash := Profile{ID: "profile:git-bash", Name: "Git Bash", Command: []string{`C:\Program Files\Git\bin\bash.exe`, "-i", "-l"}}
	nu := Profile{ID: "profile:nu", Name: "Nushell", Command: []string{"nu"}, Env: []string{"FOO=bar"}}
	for _, p := range []Profile{bash, nu} {
		if err := set.PutProfile(p, nil); err != nil {
			t.Fatal(err)
		}
	}
	bash.Name, bash.Folder = "Bash", `C:\src`
	if err := set.PutProfile(bash, nil); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(list []Profile) []string {
		var out []string
		for _, p := range list {
			out = append(out, p.ID)
		}
		return out
	}
	got := again.Profiles()
	if want := []string{"profile:git-bash", "profile:nu"}; !slices.Equal(ids(got), want) {
		t.Fatalf("read back, the profiles are %v, want %v", ids(got), want)
	}
	if got[0].Name != "Bash" || got[0].Folder != `C:\src` || !slices.Equal(got[1].Env, nu.Env) {
		t.Fatalf("read back, the profiles are %+v", got)
	}
	if err := again.DropProfile("profile:git-bash"); err != nil {
		t.Fatal(err)
	}
	if got := ids(again.Profiles()); !slices.Equal(got, []string{"profile:nu"}) {
		t.Fatalf("one removed, the profiles are %v", got)
	}
}

// A profile that names nothing to open, or an id twice over, is turned
// away on the way in and on the way out.
func TestAProfileThatCannotBeOpenedIsRefused(t *testing.T) {
	for _, c := range []struct {
		what string
		body string
		put  []Profile
	}{
		{"no id", `{"version":1,"profiles":[{"id":"","name":"x","command":["nu"]}]}`,
			[]Profile{{Name: "x", Command: []string{"nu"}}}},
		{"no command", `{"version":1,"profiles":[{"id":"profile:x","name":"x","command":[]}]}`,
			[]Profile{{ID: "profile:x", Name: "x"}}},
		{"an empty program", `{"version":1,"profiles":[{"id":"profile:x","name":"x","command":[""]}]}`,
			[]Profile{{ID: "profile:x", Name: "x", Command: []string{""}}}},
		{"an id twice", `{"version":1,"profiles":[{"id":"profile:x","name":"x","command":["nu"]},{"id":"profile:x","name":"y","command":["sh"]}]}`,
			nil},
	} {
		path := at(t)
		if err := os.WriteFile(path, []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Errorf("a file with %s loaded", c.what)
		}
		set := settingsAt(t)
		for _, p := range c.put {
			if err := set.PutProfile(p, nil); err == nil {
				t.Errorf("a profile with %s was saved", c.what)
			}
		}
	}
}

// A setting a newer kakel wrote is still there after a profile is
// saved.
func TestSavingAProfileKeepsASettingFromANewerKakel(t *testing.T) {
	path := at(t)
	if err := os.WriteFile(path, []byte(`{"version": 1, "andThis": "x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := set.PutProfile(Profile{ID: "profile:nu", Name: "Nushell", Command: []string{"nu"}}, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"andThis": "x"`) || !strings.Contains(string(raw), `"profile:nu"`) {
		t.Fatalf("written back:\n%s", raw)
	}
}

// A server's profile comes back on its server, and the profile its new
// terminal runs comes back until it is cleared, or the profile or the
// server's profiles are removed.
func TestAServersShellIsOneOfItsProfiles(t *testing.T) {
	path := at(t)
	set, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []Profile{
		{ID: "profile:tmux", Name: "tmux", HostID: "srv", Command: []string{"tmux", "new", "-A"}},
		{ID: "profile:htop", Name: "htop", HostID: "srv", Command: []string{"htop"}},
		{ID: "profile:nu", Name: "Nushell", Command: []string{"nu"}},
	} {
		if err := set.PutProfile(p, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := set.PutServerShell("srv", "profile:tmux"); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Profiles()[0].HostID; got != "srv" {
		t.Fatalf("read back, the profile is on %q", got)
	}
	if got := again.ServerShell("srv"); got != "profile:tmux" {
		t.Fatalf("read back, the server runs %q", got)
	}
	if err := again.PutServerShell("srv", ""); err != nil {
		t.Fatal(err)
	}
	if got := again.ServerShells(); len(got) != 0 {
		t.Fatalf("cleared, the servers run %v", got)
	}
	if err := again.PutServerShell("srv", "profile:tmux"); err != nil {
		t.Fatal(err)
	}
	if err := again.DropProfile("profile:tmux"); err != nil {
		t.Fatal(err)
	}
	if got := again.ServerShell("srv"); got != "" {
		t.Fatalf("its profile removed, the server runs %q", got)
	}
	if err := again.PutServerShell("srv", "profile:htop"); err != nil {
		t.Fatal(err)
	}
	if err := again.DropProfilesOn("srv"); err != nil {
		t.Fatal(err)
	}
	if got, left := again.ServerShell("srv"), again.Profiles(); got != "" || len(left) != 1 || left[0].ID != "profile:nu" {
		t.Fatalf("its profiles removed, the server runs %q and the profiles are %+v", got, left)
	}
}

// A server can run only a profile on that server: one that is not
// there, or is this computer's or another server's, is turned away on
// the way in and on the way out.
func TestAServersShellThatIsNotItsProfileIsRefused(t *testing.T) {
	profiles := `"profiles":[{"id":"profile:nu","name":"Nushell","command":["nu"]},` +
		`{"id":"profile:top","name":"top","hostId":"other","command":["top"]}]`
	for _, c := range []struct {
		what, host, id string
	}{
		{"no such profile", "srv", "profile:gone"},
		{"this computer's profile", "srv", "profile:nu"},
		{"another server's profile", "srv", "profile:top"},
		{"no server", "", "profile:nu"},
	} {
		path := at(t)
		body := `{"version":1,` + profiles + `,"serverShells":{"` + c.host + `":"` + c.id + `"}}`
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Errorf("a file naming %s loaded", c.what)
		}
		if err := os.WriteFile(path, []byte(`{"version":1,`+profiles+`}`), 0o600); err != nil {
			t.Fatal(err)
		}
		set, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := set.PutServerShell(c.host, c.id); err == nil {
			t.Errorf("%s was saved", c.what)
		}
	}
}
