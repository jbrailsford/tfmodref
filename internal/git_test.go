package internal

import "testing"

var remotes = []string{
	// "git@github.com:terraform-aws-modules/terraform-aws-vpc.git",
	"https://github.com/terraform-aws-modules/terraform-aws-vpc.git",
}

func TestGetRemote(t *testing.T) {
	for _, remote := range remotes {
		_, err := RemoteTags(remote)
		if err != nil {
			t.Errorf("error getting tags for '%s': %s", remote, err)
		}
	}
}
