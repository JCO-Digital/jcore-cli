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
		ok       bool
	}{
		{[]string{"about"}, "", "", "", false, 0, true},
		{[]string{"about"}, "", "page", "fi", true, 0, true},
		{[]string{"about"}, "", "", "", false, 12, true},
		{nil, "main", "", "", false, 0, true},
		{[]string{"about"}, "main", "", "", false, 0, false},
		{nil, "", "", "", false, 0, false},
		{nil, "main", "page", "", false, 0, false},
		{nil, "main", "", "fi", false, 0, false},
		{nil, "main", "", "", true, 0, false},
		{nil, "main", "", "", false, 12, false},
		{[]string{"about"}, "", "", "", false, -1, false},
	}
	for _, c := range cases {
		err := validateSyncArgs(c.args, c.menu, c.postType, c.lang, c.meta, c.remoteID)
		if (err == nil) != c.ok {
			t.Errorf("validateSyncArgs(%v, %q, %q, %q, %v, %d) = %v", c.args, c.menu, c.postType, c.lang, c.meta, c.remoteID, err)
		}
	}
}
