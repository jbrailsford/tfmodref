package internal

import (
	"net/url"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"
)

var (
	unversionedSource GitSource
	versionedSource   GitSource
)

func init() {
	unversionedSourceURL, _ := url.Parse("git::git@github.com:terraform-aws-modules/terraform-aws-vpc.git")
	versionedSourceURL, _ := url.Parse("git::git@github.com:terraform-aws-modules/terraform-aws-vpc.git?ref=v3.0.0")
	remoteURL, _ := url.Parse("ssh://git@github.com:terraform-aws-modules/terraform-aws-vpc.git")

	unversionedSource = GitSource{
		localVersion:        nil,
		LatestRemoteVersion: semver.MustParse("v5.0.0"),
		RemoteVersions: semver.Collection{
			semver.MustParse("v1.0.0"),
			semver.MustParse("v2.0.0"),
			semver.MustParse("v3.0.0"),
			semver.MustParse("v4.0.0"),
			semver.MustParse("v5.0.0"),
		},
		LocalVersionIsMain: true,
		BlockIndex:         0,
		SourceURL:          unversionedSourceURL,
		RemoteURL:          remoteURL,
		Prefixes:           []string{"git"},
	}

	versionedSource = GitSource{
		localVersion:        semver.MustParse("v3.0.0"),
		LatestRemoteVersion: semver.MustParse("v5.0.0"),
		RemoteVersions: semver.Collection{
			semver.MustParse("v1.0.0"),
			semver.MustParse("v2.0.0"),
			semver.MustParse("v3.0.0"),
			semver.MustParse("v4.0.0"),
			semver.MustParse("v5.0.0"),
		},
		LocalVersionIsMain: false,
		BlockIndex:         0,
		SourceURL:          versionedSourceURL,
		RemoteURL:          remoteURL,
		Prefixes:           []string{"git"},
	}
}

func TestLocalVersionString(t *testing.T) {
	assert.Equal(t, unversionedSource.LocalVersionString(), "HEAD", "local version string should be HEAD when no local version set")
	assert.Equal(t, versionedSource.LocalVersionString(), "v3.0.0", "local version string should not be HEAD when version set")
}

func TestDowngradeDetection(t *testing.T) {
	downgradeVersion := semver.MustParse("v0.0.0")
	upgradeVersion := semver.MustParse("v10.0.0")
	assert.True(t, versionedSource.WouldForceDowngrade(downgradeVersion), "should detect downgrade when target version lower than current local version")
	assert.False(t, versionedSource.WouldForceDowngrade(upgradeVersion), "should not detect downgrade when target version lower than current local version")
}

func TestVersionMatch(t *testing.T) {
	correctVersion := semver.MustParse("v3.0.0")
	incorrectVersion := semver.MustParse("v2.0.0")
	assert.True(t, versionedSource.IsVersion(correctVersion), "should return true if input version is equal to local version")
	assert.False(t, versionedSource.IsVersion(incorrectVersion), "should return false if input version is not equal to local version")
}

func TestTagSearchingWithConstraint(t *testing.T) {
	upgradeConstraint, _ := semver.NewConstraint("> 0.0.0")
	equalConstraint, _ := semver.NewConstraint("= 2.0.0")
	downgradeConstraint, _ := semver.NewConstraint("< 2.0.0")
	assert.Equal(t, versionedSource.FindLatestTagForConstraint(upgradeConstraint), semver.MustParse("v5.0.0"))
	assert.Equal(t, versionedSource.FindLatestTagForConstraint(equalConstraint), semver.MustParse("v2.0.0"))
	assert.Equal(t, versionedSource.FindLatestTagForConstraint(downgradeConstraint), semver.MustParse("v1.0.0"))
}
