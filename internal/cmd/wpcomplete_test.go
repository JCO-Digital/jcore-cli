package cmd

import (
	"errors"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestWPCompletionLine(t *testing.T) {
	tests := []struct {
		args       []string
		toComplete string
		want       string
	}{
		{[]string{"wp"}, "", "wp "},
		{[]string{"wp"}, "pl", "wp pl"},
		{[]string{"wp", "plugin"}, "", "wp plugin "},
		{[]string{"wp", "plugin"}, "lis", "wp plugin lis"},
		{[]string{"wp", "plugin", "list"}, "--for", "wp plugin list --for"},
	}
	for _, tt := range tests {
		if got := wpCompletionLine(tt.args, tt.toComplete); got != tt.want {
			t.Errorf("wpCompletionLine(%q, %q) = %q, want %q", tt.args, tt.toComplete, got, tt.want)
		}
	}
}

func TestParseWPCompletions(t *testing.T) {
	tests := []struct {
		name      string
		out       string
		want      []string
		wantFlags cobra.ShellCompDirective
	}{
		{
			name:      "empty falls back to files",
			out:       "",
			wantFlags: cobra.ShellCompDirectiveDefault,
		},
		{
			name:      "file token",
			out:       "<file>\n",
			wantFlags: cobra.ShellCompDirectiveDefault,
		},
		{
			name: "commands keep trailing spaces",
			out:  "activate \ndeactivate \nlist \n",
			want: []string{"activate ", "deactivate ", "list "},
			wantFlags: cobra.ShellCompDirectiveNoFileComp |
				cobra.ShellCompDirectiveNoSpace,
		},
		{
			name:      "file token with other candidates still allows files",
			out:       "something\n<file>\n",
			want:      []string{"something"},
			wantFlags: cobra.ShellCompDirectiveDefault,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, flags := parseWPCompletions(tt.out)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("completions = %#v, want %#v", got, tt.want)
			}
			if flags != tt.wantFlags {
				t.Errorf("flags = %v, want %v", flags, tt.wantFlags)
			}
		})
	}
}

func TestCompleteRunArgs(t *testing.T) {
	origRoot := projectRootForCompletion
	origWP := wpCLICompletions
	t.Cleanup(func() {
		projectRootForCompletion = origRoot
		wpCLICompletions = origWP
	})

	projectRootForCompletion = func() (string, error) { return "/proj", nil }
	wpCLICompletions = func(projectDir, line string, point int) (string, error) {
		if projectDir != "/proj" {
			t.Errorf("projectDir = %q", projectDir)
		}
		if line != "wp plugin lis" || point != len(line) {
			t.Errorf("line/point = %q/%d", line, point)
		}
		return "list \n", nil
	}

	got, flags := completeRunArgs(nil, []string{"wp", "plugin"}, "lis")
	if !reflect.DeepEqual(got, []string{"list "}) {
		t.Errorf("got = %#v", got)
	}
	wantFlags := cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	if flags != wantFlags {
		t.Errorf("flags = %v, want %v", flags, wantFlags)
	}

	got, flags = completeRunArgs(nil, nil, "w")
	if !reflect.DeepEqual(got, []string{"wp"}) || flags != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("first-arg wp: got %#v flags %v", got, flags)
	}

	got, flags = completeRunArgs(nil, nil, "bash")
	if got != nil || flags != cobra.ShellCompDirectiveDefault {
		t.Errorf("other first arg: got %#v flags %v", got, flags)
	}

	got, flags = completeRunArgs(nil, []string{"bash"}, "")
	if got != nil || flags != cobra.ShellCompDirectiveDefault {
		t.Errorf("non-wp command: got %#v flags %v", got, flags)
	}

	wpCLICompletions = func(string, string, int) (string, error) {
		return "", errors.New("container down")
	}
	got, flags = completeRunArgs(nil, []string{"wp"}, "pl")
	if got != nil || flags != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("docker error: got %#v flags %v", got, flags)
	}

	projectRootForCompletion = func() (string, error) { return "", nil }
	got, flags = completeRunArgs(nil, []string{"wp"}, "pl")
	if got != nil || flags != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("no project: got %#v flags %v", got, flags)
	}
}
