package category

import (
	"path/filepath"
	"testing"
)

// rule returns a category that matches the pdf files.
func rule() Category {
	return Category{
		Label:          "Documents",
		Directory:      "Documents",
		DirectoryMode:  ModeRelative,
		Extensions:     []string{"pdf"},
		URLPatternMode: ModeWildcard,
		BuiltIn:        true,
	}
}

// candidate returns a file that is being classified.
func candidate(path string) Candidate {
	return Candidate{Path: path}
}

// TestTheRelativeTargetFollowsTheBase verifies that a relative
// directory is joined onto the base directory, and that an absolute one
// is used as it is.
func TestTheRelativeTargetFollowsTheBase(t *testing.T) {
	base := t.TempDir()

	resolved, err := Resolve([]Candidate{candidate("file.PDF")}, []Category{rule()}, base)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved == nil {
		t.Fatal("the category was not resolved")
	}
	if want := filepath.Join(base, "Documents"); resolved.Directory != want {
		t.Errorf("the directory is %q, want %q", resolved.Directory, want)
	}

	// An absolute directory is used as it is.
	absolute := rule()
	absolute.DirectoryMode = ModeAbsolute
	absolute.Directory = filepath.Join(base, "Elsewhere")

	directory, err := Directory(base, absolute)
	if err != nil {
		t.Fatalf("resolve the directory: %v", err)
	}
	if directory != absolute.Directory {
		t.Errorf("the directory is %q, want %q", directory, absolute.Directory)
	}

	// A directory that climbs out of the base is refused.
	outside := rule()
	outside.Directory = "../outside"
	if _, err := Directory(base, outside); err == nil {
		t.Error("a directory that leaves the base was accepted")
	}
}

// TestEveryFileMustMatch verifies that a group of files is only
// classified when every one of them matches, and that the address
// patterns are honoured.
func TestEveryFileMustMatch(t *testing.T) {
	base := t.TempDir()

	// The second file matches no category.
	resolved, err := Resolve(
		[]Candidate{candidate("a.pdf"), candidate("b.bin")},
		[]Category{rule()},
		base,
	)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != nil {
		t.Errorf("the category was resolved to %v, want nothing", resolved)
	}

	// The pattern does not match the address.
	patterned := rule()
	patterned.URLPatterns = []string{"https://*.example.test/*"}

	resolved, err = Resolve([]Candidate{candidate("a.pdf")}, []Category{patterned}, base)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != nil {
		t.Errorf("the category was resolved to %v, want nothing", resolved)
	}

	// The pattern matches the address.
	matching := Candidate{
		Path: "a.pdf",
		URLs: []string{"https://cdn.example.test/download"},
	}
	resolved, err = Resolve([]Candidate{matching}, []Category{patterned}, base)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved == nil {
		t.Error("the category was not resolved")
	}

	// A pattern that is not a valid expression is refused.
	broken := rule()
	broken.URLPatternMode = ModeRegex
	broken.URLPatterns = []string{"("}
	if err := Validate([]Category{broken}, base); err == nil {
		t.Error("a broken pattern was accepted")
	}
}

// TestTheOrderAndTheMixedDestinationsFollowOnePolicy verifies that the
// first category that matches wins, and that files wanting different
// folders are not classified at all.
func TestTheOrderAndTheMixedDestinationsFollowOnePolicy(t *testing.T) {
	base := t.TempDir()

	first := rule()
	second := rule()
	second.Directory = "Elsewhere"

	// The first category wins.
	resolved, err := Resolve([]Candidate{candidate("a.pdf")}, []Category{first, second}, base)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved == nil {
		t.Fatal("the category was not resolved")
	}
	if want := filepath.Join(base, "Documents"); resolved.Directory != want {
		t.Errorf("the directory is %q, want %q", resolved.Directory, want)
	}

	// The second category wants a different folder, so the download
	// goes to the default directory.
	second.Extensions = []string{"txt"}
	files := []Candidate{candidate("a.pdf"), candidate("b.txt")}

	resolved, err = Resolve(files, []Category{first, second}, base)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != nil {
		t.Errorf("the category was resolved to %v, want nothing", resolved)
	}

	// The second category wants the same folder, so the download is
	// classified.
	second.Directory = "Documents/"
	resolved, err = Resolve(files, []Category{first, second}, base)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved == nil {
		t.Error("the category was not resolved")
	}

	// Nothing to classify.
	resolved, err = Resolve(nil, nil, base)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != nil {
		t.Errorf("the category was resolved to %v, want nothing", resolved)
	}
}

// TestTheExtensionsAreReadFromTheName verifies that the extension of a
// file is read the way the engine reads it.
func TestTheExtensionsAreReadFromTheName(t *testing.T) {
	cases := []struct {
		value string
		want  []string
	}{
		{"https://example.test/file%2Epdf?x=1", []string{"pdf"}},
		{"file%2Epdf", []string{""}},
		{"magnet:?dn=file.pdf", []string{""}},
		{".gitignore", []string{""}},
		{"archive.tar.gz", []string{"gz"}},
	}

	for _, test := range cases {
		got := Extensions(test.value)
		if len(got) != len(test.want) {
			t.Errorf("the extensions of %q are %v, want %v", test.value, got, test.want)
			continue
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Errorf("the extensions of %q are %v, want %v", test.value, got, test.want)
				break
			}
		}
	}
}

// TestThePartsOfASplitArchiveGoByTheArchive verifies that a part of a
// split archive is classified by the archive it belongs to.
func TestThePartsOfASplitArchiveGoByTheArchive(t *testing.T) {
	cases := []struct {
		value string
		want  []string
	}{
		{"backup.7z.001", []string{"001", "7z"}},
		{"https://example.test/a.zip.002", []string{"002", "zip"}},
		{"disk.z01", []string{"z01", "zip"}},
		{"disk.R00", []string{"r00", "rar"}},
		{"track.001", []string{"001"}},
		{"movie.mp4", []string{"mp4"}},
		{"photo.zip", []string{"zip"}},
	}

	for _, test := range cases {
		got := Extensions(test.value)
		if len(got) != len(test.want) {
			t.Errorf("the extensions of %q are %v, want %v", test.value, got, test.want)
			continue
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Errorf("the extensions of %q are %v, want %v", test.value, got, test.want)
				break
			}
		}
	}
}
