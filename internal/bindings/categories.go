package bindings

import (
	"github.com/incurl/bitmaher/internal/downloads/category"
)

// The category commands.
//
// The add dialog asks which folder a download belongs to, and the
// settings panel asks whether the categories are usable. The rules are
// the ones Rayburst uses, so that the two classify a download the same
// way.

// ResolveFileCategory answers which category a download belongs to.
//
// The candidates are the files the download would hold, and the answer
// is empty when no category wants them, or when they want different
// folders: the download goes to the default directory then.
func (s *AppService) ResolveFileCategory(
	candidates []category.Candidate,
	categories []category.Category,
	baseDir string,
) (*category.Category, error) {
	resolved, err := category.Resolve(candidates, categories, baseDir)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

// ValidateFileCategories answers whether the categories can be used.
//
// It is the check the settings panel runs before it saves them.
func (s *AppService) ValidateFileCategories(categories []category.Category, baseDir string) error {
	return category.Validate(categories, baseDir)
}
