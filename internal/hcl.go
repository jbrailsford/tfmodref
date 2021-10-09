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
	urlhelper "github.com/hashicorp/go-getter/helper/url"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
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
	rawSourceURL string
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

	for i, v := range blocksWithSource {
		gitSource := GitSource{
			BlockIndex: i,
		}

		// TODO: break this up and handle errors
		parsedURL, err := parseSourceURL(v.rawSourceURL)
		if err != nil {
			return nil, err
		}

		rootURL, err := splitSourceURL(parsedURL)
		if err != nil {
			return nil, err
		}

		queryString, err := url.ParseQuery(rootURL.RawQuery)
		if err != nil {
			return nil, err
		}

		if queryString.Has("ref") {
			sv, _ := semver.NewVersion(queryString.Get("ref"))
			gitSource.localVersion = sv
			queryString.Del("ref")
		} else {
			gitSource.LocalVersionIsMain = true
		}

		rootURL.RawQuery = queryString.Encode()
		gitSource.URL = rootURL

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

	return file.Close()
}

// SetSourceVersion updates the block source in memory to change the given sources' version to the version specified.
func (s *GitSource) SetSourceVersion(version *semver.Version) {
	query, _ := url.ParseQuery(s.URL.RawQuery)
	query.Add("ref", version.String())
	s.URL.RawQuery = query.Encode()
	s.localVersion = version
}

// UpdateBlockSource udpates the block source in the HCL, in memory, to match the source contained in the GitSource
func (p *HclParser) UpdateBlockSource(source *GitSource) {
	body := p.file.Body().Blocks()[source.BlockIndex].Body()
	body.SetAttributeValue("source", cty.StringVal(source.URL.String()))
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
			if url := extractGitURLFromAttribute(*block.Body(), filepath.Dir(p.filePath), "source"); url != "" {
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
					rawSourceURL: url,
				}
			}

		}
	}

	return
}

func extractGitURLFromAttribute(body hclwrite.Body, parentDirectory string, searchAttr string) (url string) {
	if attr := body.GetAttribute(searchAttr); attr != nil {
		// Build the tokens set in the HCL in order to obtain the value contained within
		tokens := attr.Expr().BuildTokens(nil)
		if value := extractTokenStringValue(tokens); value != "" {
			// Set url value if the given value is a valid git URL (SSH/HTTPS)
			url, _ = getter.Detect(value, parentDirectory, []getter.Detector{&getter.GitDetector{}})
		}
	}

	return
}

func extractTokenStringValue(tokens hclwrite.Tokens) (value string) {
	// Terraform source blocks do not allow variablse and are comprised of TokenOQuote + TokenQuotedLit + TokenCQuote,
	// given this, we only care about three part tokens, and in particularly the TokenQuotedLit
	if len(tokens) != 3 {
		return
	}

	if tokens[0].Type == hclsyntax.TokenOQuote && tokens[2].Type == hclsyntax.TokenCQuote {
		value = string(tokens[1].Bytes)
	}

	return
}

// Everything below here taken (with love) from https://github.com/gruntwork-io/terragrunt/blob/master/cli/tfsource/types.go
var forcedRegexp = regexp.MustCompile(`^([A-Za-z0-9]+)::(.+)$`)

func parseSourceURL(source string) (*url.URL, error) {
	forcedGetters := []string{}
	// Continuously strip the forced getters until there is no more. This is to handle complex URL schemes like the
	// git-remote-codecommit style URL.
	forcedGetter, rawSourceURL := getForcedGetter(source)
	for forcedGetter != "" {
		// Prepend like a stack, so that we prepend to the URL scheme in the right order.
		forcedGetters = append([]string{forcedGetter}, forcedGetters...)
		forcedGetter, rawSourceURL = getForcedGetter(rawSourceURL)
	}

	// Parse the URL without the getter prefix
	canonicalSourceURL, err := urlhelper.Parse(rawSourceURL)
	if err != nil {
		return nil, err
	}

	// Reattach the "getter" prefix as part of the scheme
	for _, forcedGetter := range forcedGetters {
		canonicalSourceURL.Scheme = fmt.Sprintf("%s::%s", forcedGetter, canonicalSourceURL.Scheme)
	}

	return canonicalSourceURL, nil
}

func getForcedGetter(sourceURL string) (string, string) {
	if matches := forcedRegexp.FindStringSubmatch(sourceURL); matches != nil && len(matches) > 2 {
		return matches[1], matches[2]
	}

	return "", sourceURL
}

// Modified from original source, as we don't get about the suffixed folder path,
// only the whole versioned item.
func splitSourceURL(sourceURL *url.URL) (*url.URL, error) {
	pathSplitOnDoubleSlash := strings.SplitN(sourceURL.Path, "//", 2)

	if len(pathSplitOnDoubleSlash) > 1 {
		sourceURL.Path = pathSplitOnDoubleSlash[0]
	}

	return sourceURL, nil

}
