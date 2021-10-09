package cmd

import (
	"fmt"
	"os"

	"github.com/jbrailsford/tfmodref/internal"
	"github.com/jbrailsford/tfmodref/util"
	"github.com/spf13/cobra"
)

var listRemote bool

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists the versions of the given module('s)",
	Long: `By default lists the local version (in source) of each module in the specified file/folder tree.
	
Optionally, the remote flag may be provided which will obtain the latest remote version for that module.`,
	Run: executeList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolVarP(&listRemote, "remote", "r", false, "obtain latest remote version for any found modules")
}

func executeList(cmd *cobra.Command, args []string) {
	paths, err := util.FindTerraformFiles(path, &tfExtensions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error walking path at %s with extensions [%s] (%s)", path, tfExtensions.AsCommaSeparatedString(), err.Error())
	}

	for _, path := range paths {
		parser, errs := internal.NewHclParser(path)
		if errs != nil {
			fmt.Fprintf(os.Stderr, "errors occured whilst parsing file at %s:\n", path)
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "%s\n", e.Error())
			}
			continue
		}

		sourcesInFile, err := parser.FindGitSources(listRemote)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file at %s (%s)", path, err.Error())
			continue
		}

		for module, gitVersion := range sourcesInFile {
			if listRemote {
				fmt.Printf("module: %s (local: %s, remote: %s - total versions: %d)\n", module, gitVersion.LocalVersionString(), gitVersion.LatestRemoteVersion, len(gitVersion.RemoteVersions))
			} else {
				fmt.Printf("module: %s (local: %s)\n", module, gitVersion.LocalVersionString())
			}
		}
	}
}
