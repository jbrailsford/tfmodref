package util

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// FileExtensions is warpper type to facilate simplified file extensions
// checking
type FileExtensions map[string]interface{}

// Contains searches the defined list of file extensions and checks if the
// provided extensions exists
func (f *FileExtensions) Contains(extension string) bool {
	_, ok := (*f)[extension]

	return ok
}

// AsCommaSeparatedString returns all keys in the FileExtensions map in a comma
// separated string.
func (f *FileExtensions) AsCommaSeparatedString() (s string) {
	for k := range *f {
		s = fmt.Sprintf("%s, ", k)
	}

	s = strings.TrimRight(s, ", ")

	return
}

// FindTerraformFiles walks the current directory and finds files matching
// the defined terraform file extensions
func FindTerraformFiles(basePath string, extensions *FileExtensions) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if extensions.Contains(filepath.Ext(d.Name())) {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}

			paths = append(paths, absPath)
		}

		return nil
	})

	return paths, err
}
