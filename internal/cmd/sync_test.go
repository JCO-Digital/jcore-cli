package cmd

import "testing"

func TestValidateSyncArgs(t *testing.T) {
	cases := []struct {
		args     []string
		menu     string
		postType string
		lang     string
		meta     bool
		remoteID int
		localID  int
		pull     bool
		ok       bool
	}{
		{[]string{"about"}, "", "", "", false, 0, 0, false, true},
		{[]string{"about"}, "", "page", "fi", true, 0, 0, false, true},
		{[]string{"about"}, "", "", "", false, 12, 0, false, true},
		{nil, "main", "", "", false, 0, 0, false, true},
		{[]string{"about"}, "main", "", "", false, 0, 0, false, false},
		{nil, "", "", "", false, 0, 0, false, false},
		{nil, "main", "page", "", false, 0, 0, false, false},
		{nil, "main", "", "fi", false, 0, 0, false, false},
		{nil, "main", "", "", true, 0, 0, false, false},
		{nil, "main", "", "", false, 12, 0, false, false},
		{[]string{"about"}, "", "", "", false, -1, 0, false, false},
		{[]string{"about"}, "", "", "", false, 0, 12, false, false},
		{[]string{"about"}, "", "page", "fi", true, 0, 0, true, true},
		{[]string{"about"}, "", "", "", false, 0, 12, true, true},
		{nil, "main", "", "", false, 0, 0, true, true},
		{nil, "", "", "", false, 0, 0, true, false},
		{nil, "main", "", "", false, 0, 12, true, false},
		{[]string{"about"}, "", "", "", false, 12, 0, true, false},
		{[]string{"about"}, "", "", "", false, 0, -1, true, false},
	}
	for _, c := range cases {
		err := validateSyncArgs(c.args, c.menu, c.postType, c.lang, c.meta, c.remoteID, c.localID, c.pull)
		if (err == nil) != c.ok {
			t.Errorf("validateSyncArgs(%v, %q, %q, %q, %v, %d, %d, %v) = %v", c.args, c.menu, c.postType, c.lang, c.meta, c.remoteID, c.localID, c.pull, err)
		}
	}
}
