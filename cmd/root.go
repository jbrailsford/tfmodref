package cmd

import (
	"github.com/jbrailsford/tfmodref/util"
	"github.com/spf13/cobra"
)

var (
	path         string
	tfExtensions util.FileExtensions
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tfmodref",
	Short: "A utility for working with terraform/terragrunt semver tagged modules stored in git",
	Long: `A utility for working with terraform/terragrunt semver tagged modules stored in git.
	
Provides the funcationality to obtain details of modules in use locally, available remotely, and
upgrade/downgrade, both within a semver constraint or to the latest available version.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&path, "path", "p", ".", "path to search in (recursively) for terraform files - may be an exact file or a directory")
	extensions := rootCmd.PersistentFlags().StringSliceP("extensions", "e", []string{".hcl", ".tf"}, "file extensions of files to search in for references")

	tfExtensions = make(util.FileExtensions)
	for _, ext := range *extensions {
		tfExtensions[ext] = nil
	}

	if path == "" {
		path = "."
	}

	if len(*extensions) == 0 {
		*extensions = []string{".hcl", ".tf"}
	}

	handleCobraError(rootCmd.MarkPersistentFlagDirname("path"))
	handleCobraError(rootCmd.MarkPersistentFlagFilename("path"))
}

func handleCobraError(err error) {
	if err != nil {
		util.ErrorAndExit("an error occured starting the applicaiton (%s)", err.Error())
	}
}
