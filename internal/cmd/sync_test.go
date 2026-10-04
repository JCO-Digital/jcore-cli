package cmd

import "testing"

func TestValidateSyncArgs(t *testing.T) {
	cases := []struct {
		args     []string
		menu     string
		postType string
		lang     string
		meta     bool
		ok       bool
	}{
		{[]string{"about"}, "", "", "", false, true},
		{[]string{"about"}, "", "page", "fi", true, true},
		{nil, "main", "", "", false, true},
		{[]string{"about"}, "main", "", "", false, false},
		{nil, "", "", "", false, false},
		{nil, "main", "page", "", false, false},
		{nil, "main", "", "fi", false, false},
		{nil, "main", "", "", true, false},
	}
	for _, c := range cases {
		err := validateSyncArgs(c.args, c.menu, c.postType, c.lang, c.meta)
		if (err == nil) != c.ok {
			t.Errorf("validateSyncArgs(%v, %q, %q, %q, %v) = %v", c.args, c.menu, c.postType, c.lang, c.meta, err)
		}
	}
}
