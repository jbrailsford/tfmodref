package cmd

import (
	"fmt"
	"os"

	"github.com/Masterminds/semver"
	"github.com/jbrailsford/tfmodref/internal"
	"github.com/jbrailsford/tfmodref/util"
	"github.com/spf13/cobra"
)

var (
	updateToLatest     bool
	versionUnversioned bool
	allowDowngrades    bool
	dryRun             bool
	constraintStr      string
	specifiedVersion   string
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Updates the versions of the given module('s)",
	Long: `Updates the module version (in source) of each module in the specified file/folder tree.
	
Target version may be set by specifiying a specific version, a version constraint, or requesting that the latest be used.
If not using --latest, the version will be updated without checking if it exists in the git repository.`,
	Run: executeUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().BoolVar(&updateToLatest, "latest", false, "update to latest available version")
	updateCmd.Flags().BoolVar(&versionUnversioned, "version-unversioned", false, "set a version (latest remote) on sources that are tracking HEAD")
	updateCmd.Flags().BoolVar(&allowDowngrades, "allow-downgrades", false, "allow downgrades if the current version is greater than the constraint")
	updateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "output what would change, without making any changes")
	updateCmd.Flags().StringVarP(&constraintStr, "constraint", "c", "", "semver constraint to control upgrade path, e.g., >= 1.x < 3.0.1")
	updateCmd.Flags().StringVarP(&specifiedVersion, "version", "v", "", "update to specified version, will not check if version exists")
}

func executeUpdate(cmd *cobra.Command, args []string) {
	paths, err := util.FindTerraformFiles(path, &tfExtensions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error walking path at %s with extensions [%s] (%s)", path, tfExtensions.AsCommaSeparatedString(), err.Error())
	}

	var constraint *semver.Constraints
	if constraintStr != "" {
		constraint, _ = semver.NewConstraint(constraintStr)
	}

	var version *semver.Version
	if specifiedVersion != "" {
		if version, _ = semver.NewVersion(specifiedVersion); version == nil {
			util.ErrorAndExit("specified version string %s is invalid\n", specifiedVersion)
		}
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

		sourcesInFile, err := parser.FindGitSources(version == nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file at %s (%s)", path, err.Error())
			continue
		}

		for module, gitVersion := range sourcesInFile {
			gitVersion := gitVersion

			if gitVersion.LocalVersionString() == "HEAD" && !versionUnversioned {
				fmt.Printf("skipping: %s (unversioned module, to force versioning re-run with --version-unversioned\n", module)
				continue
			}

			var targetVersion *semver.Version
			if version != nil {
				targetVersion = version
				goto update
			} else {
				targetVersion = gitVersion.LatestRemoteVersion
			}

			if constraint != nil {
				if matchedVersion := gitVersion.FindLatestTagForConstraint(constraint); matchedVersion != nil {
					targetVersion = matchedVersion
				}
			}

		update:
			if gitVersion.IsVersion(targetVersion) {
				continue
			}

			if gitVersion.WouldForceDowngrade(targetVersion) && !allowDowngrades {
				fmt.Printf("skipping: %s (target version %s is less than current version %s)\n", module, targetVersion, gitVersion.LocalVersionString())
				continue
			}

			if dryRun {
				fmt.Printf("would update: %s (from: %s, to: %s)\n", module, gitVersion.LocalVersionString(), targetVersion)
				continue
			}

			fmt.Printf("updating: %s (from: %s, to: %s)\n", module, gitVersion.LocalVersionString(), targetVersion)
			gitVersion.SetSourceVersion(targetVersion)
			parser.UpdateBlockSource(&gitVersion)
		}

		if !dryRun {
			if err := parser.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "error saving file at %s (%s)", path, err.Error())
			}
		}
	}
}
