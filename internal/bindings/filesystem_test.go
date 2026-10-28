package bindings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAPathThatWasNeverThereIsNotAFailure verifies that removing a
// path that is not there is not an error.
//
// The outcome the caller wanted has been reached, so the call answers
// that nothing was there.
func TestAPathThatWasNeverThereIsNotAFailure(t *testing.T) {
	service := &AppService{}

	removed, err := service.DeletePath(filepath.Join(t.TempDir(), "never.bin"), DeletionPermanent)
	if err != nil {
		t.Fatalf("delete a missing path: %v", err)
	}
	if removed {
		t.Error("a missing path was reported as removed")
	}
}

// TestAnEmptyPathIsNotRemoved verifies that an empty path is refused.
func TestAnEmptyPathIsNotRemoved(t *testing.T) {
	service := &AppService{}

	removed, err := service.DeletePath("  ", DeletionPermanent)
	if err != nil {
		t.Fatalf("delete an empty path: %v", err)
	}
	if removed {
		t.Error("an empty path was reported as removed")
	}
}

// TestAFolderIsRemovedWithWhatIsInIt verifies that the permanent
// removal of a folder takes everything in it.
func TestAFolderIsRemovedWithWhatIsInIt(t *testing.T) {
	directory := t.TempDir()
	nested := filepath.Join(directory, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatalf("create the folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.bin"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	service := &AppService{}

	removed, err := service.DeletePath(directory, DeletionPermanent)
	if err != nil {
		t.Fatalf("delete the folder: %v", err)
	}
	if !removed {
		t.Error("the folder was not reported as removed")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Error("the folder is still there")
	}
}

// TestTheNameOfAFileIsChecked verifies that the names a file may not
// have are refused.
func TestTheNameOfAFileIsChecked(t *testing.T) {
	refused := []string{
		"",
		"   ",
		"folder/file.bin",
		`folder\file.bin`,
		".",
		"..",
		"CON",
		"nul",
	}

	for _, name := range refused {
		if err := validateFilename(name); err == nil {
			t.Errorf("the name %q was accepted", name)
		}
	}

	accepted := []string{"file.bin", "a file.bin", "ünïcode.bin", "file.7z.001"}
	for _, name := range accepted {
		if err := validateFilename(name); err != nil {
			t.Errorf("the name %q was refused: %v", name, err)
		}
	}
}

// TestTheStateOfAFileIsRead verifies that the state of a path on the
// disk is worked out.
func TestTheStateOfAFileIsRead(t *testing.T) {
	directory := t.TempDir()

	present := filepath.Join(directory, "there.bin")
	if err := os.WriteFile(present, []byte("x"), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	if state := fileState(present); state != FileAvailable {
		t.Errorf("the state is %q, want %q", state, FileAvailable)
	}

	if state := fileState(filepath.Join(directory, "gone.bin")); state != FileMissing {
		t.Errorf("the state is %q, want %q", state, FileMissing)
	}

	if state := fileState(""); state != FileUnknown {
		t.Errorf("the state is %q, want %q", state, FileUnknown)
	}
}

// TestTheBlocklistIsCleaned verifies that the rules of a blocklist are
// read the way the engine reads them.
//
// The comments are left out, the rules that are not addresses are
// refused, and the ones that are there twice are kept once.
func TestTheBlocklistIsCleaned(t *testing.T) {
	input := "# a comment\n203.0.113.7\n203.0.113.0/24\n2001:db8::7\n2001:db8::/32\n203.0.113.7\nnot an address\n"

	content, count, err := normaliseBlocklist(input, "https://example.test/blocklist.txt")
	if err != nil {
		t.Fatalf("normalise the blocklist: %v", err)
	}

	if count != 4 {
		t.Errorf("the list holds %d rules, want 4", count)
	}
	if want := "# Source: https://example.test/blocklist.txt\n"; content[:len(want)] != want {
		t.Errorf("the list starts with %q, want the source", content[:len(want)])
	}
	for _, rule := range []string{"203.0.113.7\n", "203.0.113.0/24\n", "2001:db8::7\n", "2001:db8::/32\n"} {
		if !strings.Contains(content, rule) {
			t.Errorf("the list does not hold %q", rule)
		}
	}
	if strings.Contains(content, "not an address") {
		t.Error("the list holds a rule that is not an address")
	}
}
