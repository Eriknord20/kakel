package app

import (
	"image"
	"reflect"
	"sync"
	"testing"

	"github.com/marrasen/kakel/clip"
)

// testBoard is the clipboard the tests use in place of the system's.
var testBoard struct {
	mu   sync.Mutex
	text string
	png  []byte
}

// onAClipboardOfTheirOwn points the program's side at testBoard.
//
// The window's side copies through its gunim window, and an offscreen
// window has a clipboard of its own. The program's side reaches the
// system's clipboard through clip, in four places: clearing a copied
// secret as the window closes, reading an image to paste, checking
// for one when a middle click finds no text, and putting an image
// another window sent. A test that reached one of them with the real
// ones in place would read or replace what the user had copied.
func onAClipboardOfTheirOwn() {
	readClipboard = func() (string, error) {
		testBoard.mu.Lock()
		defer testBoard.mu.Unlock()
		return testBoard.text, nil
	}
	writeClipboard = func(s string) error {
		testBoard.mu.Lock()
		defer testBoard.mu.Unlock()
		testBoard.text = s
		return nil
	}
	readImage = func() (image.Image, bool, error) { return nil, false, nil }
	takeImage = func(png []byte) error {
		testBoard.mu.Lock()
		defer testBoard.mu.Unlock()
		testBoard.png = append([]byte(nil), png...)
		return nil
	}
}

// Every way the program's side reaches a clipboard leads to the tests'
// own. A test once logged "Prompt copied" on Windows, and a copy that
// reached the system would have replaced what the user had copied.
func TestTheTestsCopyToAClipboardOfTheirOwn(t *testing.T) {
	for what, pair := range map[string][2]any{
		"reading text":     {readClipboard, clip.Text},
		"writing text":     {writeClipboard, clip.SetText},
		"reading an image": {readImage, clip.Image},
		"putting an image": {takeImage, clip.SetPNG},
	} {
		if reflect.ValueOf(pair[0]).Pointer() == reflect.ValueOf(pair[1]).Pointer() {
			t.Errorf("%s goes to the system's clipboard", what)
		}
	}
}

// A program that sends bytes that are not text with OSC 52 is told
// nothing was copied, rather than that it was: the clipboard holds
// text, and Windows refuses a NUL outright.
func TestAProgramCopyingBytesIsToldNothingWasCopied(t *testing.T) {
	a := fontApp(t)
	for _, sent := range []string{"\x30\x82\x11\x64", "key\x00store"} {
		a.st.Notices = nil
		a.programCopied("p1", sent)
		if len(a.st.Notices) != 1 || a.st.Notices[0].Title != "Not copied" || a.st.Notices[0].Clipboard != "" {
			t.Fatalf("copying %q, the notices are %+v", sent, a.st.Notices)
		}
	}
	a.st.Notices = nil
	a.programCopied("p1", "MIIRZAIBAzCCEQ4G")
	if len(a.st.Notices) != 1 || a.st.Notices[0].Title != "Copied to the clipboard" || a.st.Notices[0].Clipboard != "MIIRZAIBAzCCEQ4G" {
		t.Fatalf("copying text, the notices are %+v", a.st.Notices)
	}
}
