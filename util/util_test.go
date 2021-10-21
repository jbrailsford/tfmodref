package util

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

var extensions = FileExtensions{
	".hcl": nil,
	".tf":  nil,
}

func TestFileExtensionsContains(t *testing.T) {
	assert.True(t, extensions.Contains(".hcl"), "validate FileExtensions contains value")
}

func TestFileExtensionsToCsvConversion(t *testing.T) {
	assert.Equal(t, ".hcl, .tf", extensions.AsCommaSeparatedString(), "validate FileExtensions can be converted to a csv string")
}

func TestErrorAndExit(t *testing.T) {
	if os.Getenv("TEST_EXIT") == "1" {
		ErrorAndExit("testing")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestErrorAndExit")
	cmd.Env = append(os.Environ(), "TEST_EXIT=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}

	t.Fatalf("process ran with err %v, want exit status 1", err)
}
