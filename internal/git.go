package internal

import (
	"github.com/Masterminds/semver"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
)

// RemoteTags returns a colelction of SemVer tags, if the tags are not in SemVer
// format and Error is returned.
func RemoteTags(repositoryURL string) (semver.Collection, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repositoryURL},
	})

	var tags semver.Collection
	refs, err := remote.List(&git.ListOptions{})

	if err != nil {
		return nil, err
	}

	for _, ref := range refs {
		if ref.Name().IsTag() {
			version, err := semver.NewVersion(ref.Name().Short())
			if err != nil {
				return nil, err
			}

			tags = append(tags, version)
		}
	}

	return tags, err
}

type sourceCache map[string]semver.Collection

// SourceCache is a global cache of repo URL's and available remote versions,
// used to reduce network calls to find verisons.
var SourceCache sourceCache

func init() {
	SourceCache = make(map[string]semver.Collection)
}

func (sc sourceCache) Get(url string) semver.Collection {
	if val, ok := sc[url]; ok {
		return val
	}

	return nil
}

func (sc sourceCache) Set(url string, collection semver.Collection) {
	sc[url] = collection
}
