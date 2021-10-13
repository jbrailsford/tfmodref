package internal

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	urlhelper "github.com/hashicorp/go-getter/helper/url"
)

// TerragruntBlockType denotes the parent block type for a source ref in a terragrunt file
const TerragruntBlockType string = "terraform"

// TerraformBlockType denotes the parent block type for a source ref in a terraform file
const TerraformBlockType string = "module"

// HclParser presides over a given HCL formatted file and can be used to both read and udpate it.
type HclParser struct {
	filePath string
	file     *hclwrite.File
}

// BlockSource contains the name of a given module containing a source ref,in the case of
// terraform this is file path + module name, in the case of terragrunt it's the filepath only.
// This also contains the raw URL extracted from that block.
type BlockSource struct {
	Name         string
	gitRemoteURL *url.URL
	sourceURL    *url.URL
	prefixes     []string
}

// NewHclParser reads in a given HCL file and instansiates a new instance of HclParser
func NewHclParser(filePath string) (*HclParser, []error) {
	parsed, errs := parseHcl(filePath)
	if errs != nil {
		// TODO: handle err properly
		return nil, errs
	}

	return &HclParser{
		filePath: filePath,
		file:     parsed,
	}, nil
}

// FindGitSources searches the current HCL for blocks which contain a `source` attribute,
// and then extracts the version references from it. Optionally, it may also retrieve
// information about the versions of the module available remotely.
func (p *HclParser) FindGitSources(includeRemote bool) (map[string]GitSource, error) {
	sources := make(map[string]GitSource)

	blocksWithSource := p.findBlocksWithGitSource()

	// probably don't need to map one struct to another here, may be cleaner to build just one.
	for i, v := range blocksWithSource {
		gitSource := GitSource{
			BlockIndex: i,
		}

		qs := v.sourceURL.Query()
		// queryString.Has exists (url.Values.Has) but GoSec can't see it for some reason and fails?
		if _, ok := qs["ref"]; ok {
			sv, _ := semver.NewVersion(qs.Get("ref"))
			gitSource.localVersion = sv
		} else {
			gitSource.LocalVersionIsMain = true
		}

		gitSource.SourceURL = v.sourceURL
		gitSource.RemoteURL = v.gitRemoteURL
		gitSource.Prefixes = v.prefixes

		if includeRemote {
			if err := gitSource.UpdateRemoteTags(); err != nil {
				fmt.Fprintf(os.Stderr, "could not get remote tags for module %s (%s)\n", v.Name, err.Error())
				continue
			}
		}

		sources[v.Name] = gitSource
	}

	return sources, nil
}

// Save updates the target file
func (p *HclParser) Save() error {
	fi, err := os.Stat(p.filePath)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(p.filePath, os.O_TRUNC|os.O_RDWR|os.O_EXCL, fi.Mode())
	if err != nil {
		return err
	}

	output := bufio.NewWriter(file)
	defer output.Flush()

	p.file.BuildTokens(nil)
	_, err = p.file.WriteTo(output)
	if err != nil {
		return err
	}

	if err := output.Flush(); err != nil {
		return err
	}

	return file.Close()
}

// UpdateBlockSource udpates the block source in the HCL, in memory, to match the source contained in the GitSource
func (p *HclParser) UpdateBlockSource(source *GitSource) {
	body := p.file.Body().Blocks()[source.BlockIndex].Body()
	body.SetAttributeValue("source", cty.StringVal(source.HCLSafeSourceURL()))
	body.BuildTokens(nil)
}

func parseHcl(filePath string) (*hclwrite.File, []error) {
	raw, err := ioutil.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, []error{err}
	}

	parsed, diags := hclwrite.ParseConfig(raw, filepath.Base(filePath), hcl.InitialPos)

	if diags.HasErrors() {
		return nil, diags.Errs()
	}

	return parsed, nil
}

func (p *HclParser) findBlocksWithGitSource() (blocksWithRefs map[int]BlockSource) {
	blocksWithRefs = make(map[int]BlockSource)
	blocks := p.file.Body().Blocks()

	for i, block := range blocks {
		// We are only interested in blocks that *can* contain a source attribute
		if block.Type() == TerraformBlockType || block.Type() == TerragruntBlockType {
			// Attempt to find the source attribtue within the block, and return if if the url is a valid git URL
			if prefixes, url, gitURL := extractGitURLFromAttribute(*block.Body(), filepath.Dir(p.filePath), "source"); url != nil && gitURL != nil {
				// Set the module name to the filepath of source hcl
				moduleName := p.filePath

				// If the source is contained within a module block (terraform only) it will also be named,
				// as such we should include that name in the metadata, since multiple modules may exist
				// within one file - this is not the case in terragrunt, in terragrunt there's only one module
				// reference per file
				if len(block.Labels()) == 1 {
					moduleName = fmt.Sprintf("%s [%s]", moduleName, block.Labels()[0])
				}

				blocksWithRefs[i] = BlockSource{
					Name:         moduleName,
					gitRemoteURL: gitURL,
					sourceURL:    url,
					prefixes:     prefixes,
				}
			}

		}
	}

	return
}

func extractGitURLFromAttribute(body hclwrite.Body, parentDirectory string, searchAttr string) ([]string, *url.URL, *url.URL) {
	attr := body.GetAttribute(searchAttr)
	if attr == nil {
		return []string{}, nil, nil
	}

	// Build the tokens set in the HCL in order to obtain the value contained within
	// TODO: make this cleaner
	tokens := attr.Expr().BuildTokens(nil)
	value := extractTokenStringValue(tokens)
	if value == "" {
		return []string{}, nil, nil
	}

	rawGitURL, err := getter.Detect(value, parentDirectory, []getter.Detector{&getter.GitDetector{}, &getter.GitHubDetector{}, &getter.GitLabDetector{}})
	if err != nil {
		return []string{}, nil, nil
	}

	prefixes, rawURL := splitSourceURLGetters(rawGitURL)
	url, err := urlhelper.Parse(rawURL)
	if err != nil {
		return []string{}, nil, nil
	}

	gitURL, err := urlhelper.Parse(rawURL)
	if err != nil {
		return []string{}, nil, nil
	}

	if strings.Contains(gitURL.Path, "//") {
		ejectGitURLFolder(gitURL)
	}

	gitURL.RawQuery = ""

	return prefixes, url, gitURL
}

func extractTokenStringValue(tokens hclwrite.Tokens) (value string) {
	// Terraform source blocks do not allow variablse and are comprised of TokenOQuote + (TokenQuotedLit * n) + TokenCQuote,
	// given this, we need to extract and combine all values betwen OQuote and CQuote.
	if tokens[0].Type == hclsyntax.TokenOQuote && tokens[len(tokens)-1].Type == hclsyntax.TokenCQuote {
		for _, token := range tokens[1 : len(tokens)-1] {
			value += string(token.Bytes)
		}
	}

	return
}

func ejectGitURLFolder(url *url.URL) {
	parts := strings.SplitN(url.Path, "//", 2)
	if len(parts) > 1 {
		(*url).Path = parts[0]
	}

}

// Everything below here taken (with love) from https://github.com/gruntwork-io/terragrunt/blob/master/cli/tfsource/types.go
// and modified garishly.
var forcedRegexp = regexp.MustCompile(`^([A-Za-z0-9]+)::(.+)$`)

func splitSourceURLGetters(source string) ([]string, string) {
	forcedGetters := []string{}
	forcedGetter, rawSourceURL := getForcedGetter(source)
	for forcedGetter != "" {
		forcedGetters = append([]string{forcedGetter}, forcedGetters...)
		forcedGetter, rawSourceURL = getForcedGetter(rawSourceURL)
	}

	return forcedGetters, rawSourceURL
}

func getForcedGetter(sourceURL string) (string, string) {
	if matches := forcedRegexp.FindStringSubmatch(sourceURL); matches != nil && len(matches) > 2 {
		return matches[1], matches[2]
	}

	return "", sourceURL
}
