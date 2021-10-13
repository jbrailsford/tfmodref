package internal

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
)

// GitSource holds the metadata about a given git source, such as it's verion,
// available remote versions, and whether it is locally versioned.
type GitSource struct {
	localVersion        *semver.Version
	LatestRemoteVersion *semver.Version
	RemoteVersions      semver.Collection
	LocalVersionIsMain  bool
	BlockIndex          int
	SourceURL           *url.URL
	RemoteURL           *url.URL
	Prefixes            []string
}

// LocalVersionString returns either `HEAD` (in the case of no local version being set)
// or it returns the current local version.
func (gs *GitSource) LocalVersionString() string {
	if gs.LocalVersionIsMain {
		return "HEAD"
	}

	return gs.localVersion.String()
}

// WouldForceDowngrade returns true of the current local version is greater than the provided
// version.
func (gs *GitSource) WouldForceDowngrade(version *semver.Version) bool {
	if gs.localVersion != nil {
		return gs.localVersion.GreaterThan(version)
	}

	return false
}

// IsVersion returns true if the source version is the same as the supplied version
func (gs *GitSource) IsVersion(version *semver.Version) bool {
	if gs.localVersion != nil {
		return gs.localVersion.Equal(version)
	}

	return false
}

// FindLatestTagForConstraint finds the latest tag in RemoteVersions matching the given
// constraint.
func (gs *GitSource) FindLatestTagForConstraint(constraint *semver.Constraints) *semver.Version {
	for i := len(gs.RemoteVersions) - 1; i >= 0; i-- {
		if constraint.Check(gs.RemoteVersions[i]) {
			return gs.RemoteVersions[i]
		}
	}

	return nil
}

// UpdateRemoteTags requests a list of git tags from the source origin, and sets
// them against this GitSource object.
func (gs *GitSource) UpdateRemoteTags() error {
	if tags := SourceCache.Get(gs.RemoteURL.String()); tags != nil {
		gs.setRemoteTags(tags)
	} else {
		tags, err := RemoteTags(gs.RemoteURL.String())
		if err != nil {
			return err
		}

		gs.setRemoteTags(tags)
		SourceCache.Set(gs.RemoteURL.String(), gs.RemoteVersions)
	}

	return nil
}

// SetSourceVersion updates the git source in memory to change the given sources' version to the version specified.
func (gs *GitSource) SetSourceVersion(version *semver.Version) {
	qs := gs.SourceURL.Query()
	qs.Set("ref", version.String())
	gs.SourceURL.RawQuery = qs.Encode()
}

func (gs *GitSource) setRemoteTags(tags semver.Collection) {
	sort.Sort(tags)

	gs.LatestRemoteVersion = tags[len(tags)-1]
	gs.RemoteVersions = tags
}

// HCLSafeSourceURL retruns a url in string form matching the original HCL source (with prefixes attached)
func (gs *GitSource) HCLSafeSourceURL() string {
	source := gs.SourceURL
	// may need to handle more prefixes here, but this covers ssh://, git::git@, git::ssh:// and git::https://
	if source.Scheme == "ssh" {
		source.Scheme = ""
	}

	url := strings.TrimLeft(source.String(), "//")
	for _, prefix := range gs.Prefixes {
		url = prefix + "::" + url
	}
	fmt.Println(url)
	return url
}
