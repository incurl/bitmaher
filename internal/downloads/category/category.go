// Package category classifies the downloads into folders.
//
// A category is a rule: a name, a directory and the extensions or the
// address patterns it matches. The resolution answers the question the
// add dialog asks: which category does this download belong to?
//
// The logic is the one Rayburst uses, so that the two applications
// classify the same download the same way.
package category

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// DirectoryMode says how a category's directory is read.
//
// "relative" means the directory is joined onto the base directory, and
// "absolute" means it is used as it is.
type DirectoryMode string

const (
	// ModeRelative joins the directory onto the base directory.
	ModeRelative DirectoryMode = "relative"
	// ModeAbsolute uses the directory as it is.
	ModeAbsolute DirectoryMode = "absolute"
)

// PatternMode says how a category's address patterns are read.
//
// "wildcard" is the shell's own, and "regex" is a regular expression.
type PatternMode string

const (
	// ModeWildcard reads a pattern as a shell wildcard.
	ModeWildcard PatternMode = "wildcard"
	// ModeRegex reads a pattern as a regular expression.
	ModeRegex PatternMode = "regex"
)

// The bounds the patterns are held to.
//
// A pattern that is longer than this cannot be a host or a path, and a
// pattern that matches a very long address is a mistake.
const (
	// MaxPatternLength is the longest a pattern may be.
	MaxPatternLength = 512
	// MaxPatternTargetLength is the longest an address may be.
	MaxPatternTargetLength = 4096
)

// Category is a rule that puts a download into a folder.
type Category struct {
	// Label is the name the category is shown under.
	Label string `json:"label"`
	// Directory is the folder the downloads of the category go to.
	Directory string `json:"directory"`
	// DirectoryMode says how the directory is read.
	DirectoryMode DirectoryMode `json:"directoryMode"`
	// Extensions are the extensions the category matches, without the
	// dot. An empty list matches every extension.
	Extensions []string `json:"extensions"`
	// URLPatterns are the address patterns the category matches.
	URLPatterns []string `json:"urlPatterns"`
	// URLPatternMode says how the patterns are read.
	URLPatternMode PatternMode `json:"urlPatternMode"`
	// BuiltIn reports whether the category is one of the application's
	// own. The user cannot remove those.
	BuiltIn bool `json:"builtIn"`
}

// Candidate is a file that is being classified.
type Candidate struct {
	// Path is the path of the file, or the address it would come from.
	Path string `json:"path"`
	// URLs are the addresses the file would be fetched from.
	URLs []string `json:"urls"`
}

// Directory works out the folder a category puts its downloads in.
//
// A relative directory is joined onto the base directory, and an
// absolute one is used as it is. A "~" at the start of the base
// directory means the home directory of the user.
func Directory(base string, category Category) (string, error) {
	directory := category.Directory

	switch category.DirectoryMode {
	case ModeAbsolute:
		if !path.IsAbs(directory) {
			return "", fmt.Errorf("the directory of a category must be an absolute path")
		}
		return directory, nil

	case ModeRelative:
		if path.IsAbs(directory) {
			return "", fmt.Errorf("the directory of a category must be a relative path")
		}
		// The directory may not climb out of the base directory.
		if !isPlainRelative(directory) {
			return "", fmt.Errorf("the directory of a category must not leave the base directory")
		}

		resolved := base
		if rest, ok := trimHome(base); ok {
			home, err := homeDirectory()
			if err != nil {
				return "", err
			}
			resolved = path.Join(home, rest)
		}
		if !path.IsAbs(resolved) {
			return "", fmt.Errorf("the default download directory must be an absolute path")
		}
		return path.Join(resolved, directory), nil

	default:
		return "", fmt.Errorf("the directory of a category does not match its mode")
	}
}

// isPlainRelative reports whether a path stays inside its base.
//
// A path that climbs out with ".." would put the downloads somewhere
// the user did not ask for.
func isPlainRelative(directory string) bool {
	if directory == "" {
		return false
	}
	for _, part := range strings.Split(filepathToSlash(directory), "/") {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return false
		}
	}
	return true
}

// trimHome removes the "~" from the start of a path.
//
// It answers whether the path was a home-relative one, and what was
// left of it.
func trimHome(value string) (string, bool) {
	for _, prefix := range []string{"~/", "~\\"} {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimLeft(value[len(prefix):], "/\\"), true
		}
	}
	return "", false
}

// Validate checks that the categories can be used.
//
// Every category must have a directory that can be resolved and
// patterns that can be compiled.
func Validate(categories []Category, base string) error {
	for _, category := range categories {
		if _, err := Directory(base, category); err != nil {
			return err
		}
		if _, err := Patterns(category); err != nil {
			return err
		}
	}
	return nil
}

// Patterns compiles the address patterns of a category.
//
// A wildcard is turned into a regular expression that matches the whole
// address, and a regular expression is used as it is.
func Patterns(category Category) ([]*regexp.Regexp, error) {
	compiled := []*regexp.Regexp{}

	for _, pattern := range category.URLPatterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		if len(pattern) > MaxPatternLength {
			return nil, fmt.Errorf("the address pattern of a category is longer than %d bytes", MaxPatternLength)
		}

		expression := pattern
		switch category.URLPatternMode {
		case ModeRegex:
			expression = pattern
		case ModeWildcard, "":
			expression = "^" + regexp.QuoteMeta(pattern) + "$"
			// The wildcard is the shell's own: a "*" stands for any
			// run of characters.
			expression = strings.ReplaceAll(expression, `\*`, ".*")
		default:
			return nil, fmt.Errorf("the address pattern mode of a category is not supported")
		}

		matcher, err := regexp.Compile("(?i)" + expression)
		if err != nil {
			return nil, fmt.Errorf("the address pattern of a category is not valid: %w", err)
		}
		compiled = append(compiled, matcher)
	}

	return compiled, nil
}

// Resolve answers which category a set of files belongs to.
//
// The files are read one by one, and the first one that no category
// matches ends the search: the download goes to the default directory
// instead. When the files match different categories, the answer is
// none as well, because one download cannot go to two folders.
func Resolve(candidates []Candidate, categories []Category, base string) (*Category, error) {
	prepared := make([]compiledCategory, 0, len(categories))
	for _, category := range categories {
		patterns, err := Patterns(category)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, compiledCategory{category: category, patterns: patterns})
	}

	var resolved *Category

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Path) == "" {
			continue
		}

		names := Extensions(candidate.Path)

		var found *compiledCategory
		for i := range prepared {
			entry := &prepared[i]
			if !matches(entry, names, candidate) {
				continue
			}
			found = entry
			break
		}

		if found == nil {
			// No category wants this file, so none of them wants
			// the download.
			return nil, nil
		}

		category := found.category
		directory, err := Directory(base, category)
		if err != nil {
			return nil, err
		}
		category.Directory = directory
		category.DirectoryMode = ModeAbsolute

		if resolved != nil && !sameDirectory(resolved.Directory, category.Directory) {
			// The files want different folders, so the download
			// goes to the default directory.
			return nil, nil
		}

		resolved = &category
	}

	return resolved, nil
}

// compiledCategory is a category with its patterns compiled.
type compiledCategory struct {
	// category is the rule itself.
	category Category
	// patterns are the address patterns, compiled.
	patterns []*regexp.Regexp
}

// matches reports whether a category wants a file.
func matches(entry *compiledCategory, names []string, candidate Candidate) bool {
	// A category with neither extensions nor patterns matches nothing:
	// it would take every download.
	if len(entry.category.Extensions) == 0 && len(entry.patterns) == 0 {
		return false
	}

	// The extensions must match, unless the category has none, which
	// means every extension.
	if len(entry.category.Extensions) > 0 {
		matched := false
		for _, extension := range entry.category.Extensions {
			extension = strings.TrimPrefix(strings.TrimSpace(extension), ".")
			for _, name := range names {
				if strings.EqualFold(extension, name) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// The patterns must match, unless the category has none.
	if len(entry.patterns) > 0 {
		matched := false
		targets := append(append([]string{}, candidate.URLs...), candidate.Path)
		for _, target := range targets {
			if len(target) > MaxPatternTargetLength {
				continue
			}
			for _, pattern := range entry.patterns {
				if pattern.MatchString(target) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// sameDirectory reports whether two paths name the same folder.
//
// The comparison follows the rules of the platform: on Windows the
// letters do not matter, and everywhere else the paths must be equal.
func sameDirectory(left, right string) bool {
	left = path.Clean(filepathToSlash(left))
	right = path.Clean(filepathToSlash(right))

	if isWindows() {
		return strings.EqualFold(left, right)
	}
	return left == right
}

// Extensions returns the extensions a file goes by.
//
// It is the file's own extension and, for a part of a split archive,
// the archive's, so that the part lands with the archive.
func Extensions(value string) []string {
	name := FileName(value)

	own := lowerExtension(name)
	extensions := []string{own}

	archive := ""
	switch {
	case isDigits(own) && len(own) == 3:
		// A part of a split archive: ".001" and the like. The stem
		// is the name without its own extension.
		stem := strings.TrimSuffix(filepathToSlash(name), path.Ext(filepathToSlash(name)))
		archive = lowerExtension(stem)
	case len(own) == 3 && own[0] == 'z' && isDigits(own[1:]):
		// A part of a split zip: ".z01" and the like.
		archive = "zip"
	case len(own) == 3 && own[0] == 'r' && isDigits(own[1:]):
		// A part of a split rar: ".r00" and the like.
		archive = "rar"
	}

	if archive != "" {
		extensions = append(extensions, archive)
	}
	return extensions
}

// FileName returns the name of a file or of the path an address points
// at.
//
// An address is asked for the last part of its path, because that is
// the name the file would have.
func FileName(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "sftp":
		// The path of the address, without the query.
		decoded, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return parsed.Path
		}
		return path.Base(decoded)
	case "magnet", "data", "blob":
		// These have no name of their own.
		return ""
	default:
		return value
	}
}

// lowerExtension returns the extension of a name, in lower case.
//
// A name that is only an extension — ".gitignore" — has none of its
// own: the extension is the whole name.
func lowerExtension(name string) string {
	base := path.Base(filepathToSlash(name))
	if base == "." || base == ".." {
		return ""
	}

	extension := path.Ext(base)
	if extension == "" {
		return ""
	}

	// The name is the extension itself when it starts with a dot and
	// holds nothing else: ".gitignore" is a name, not an extension.
	if strings.HasPrefix(base, ".") && len(base) > 1 {
		return ""
	}

	return strings.ToLower(strings.TrimPrefix(extension, "."))
}

// isDigits reports whether a string is made of digits only.
func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
