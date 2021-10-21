package util

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
func (f *FileExtensions) AsCommaSeparatedString() string {
	keys := make([]string, len(*f))

	// Could use append here, but this comes out slightly more efficient.
	i := 0
	for k := range *f {
		keys[i] = k
		i++
	}

	sort.Strings(keys)

	return strings.Join(keys, ", ")
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

// ErrorAndExit writes the given message to stderr and exits the program.
func ErrorAndExit(msg string, params ...interface{}) {
	fmt.Fprintf(os.Stderr, fmt.Sprintf("%s\n", msg), params...)
	os.Exit(1)
}
