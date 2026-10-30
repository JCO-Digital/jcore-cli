package cmd

import (
	"reflect"
	"testing"
)

func TestRunEnv(t *testing.T) {
	unset := func(string) (string, bool) { return "", false }
	set := func(name string) (string, bool) {
		if name == "XDEBUG_MODE" {
			return "debug", true
		}
		return "", false
	}

	tests := []struct {
		name       string
		keepXdebug bool
		lookupEnv  func(string) (string, bool)
		want       []string
	}{
		{name: "default turns Xdebug off", lookupEnv: unset, want: []string{"XDEBUG_MODE=off"}},
		{name: "XDEBUG_MODE from the shell is passed on", lookupEnv: set, want: []string{"XDEBUG_MODE=debug"}},
		{name: "--xdebug leaves it to php.ini", keepXdebug: true, lookupEnv: set, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runEnv(tt.keepXdebug, tt.lookupEnv); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("runEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunCommandParts(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"wp", "option", "get", "siteurl"}, []string{"wp", "option", "get", "siteurl"}},
		{[]string{"wp plugin list"}, []string{"sh", "-c", "wp plugin list"}},
		{[]string{"bash"}, []string{"bash"}},
		// Spaces inside one of several arguments belong to that argument.
		{[]string{"wp", "eval", `echo "a b";`}, []string{"wp", "eval", `echo "a b";`}},
	}
	for _, tt := range tests {
		if got := runCommandParts(tt.args); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("runCommandParts(%q) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

// The command's own flags must reach it rather than be parsed as jcore's.
func TestRunPassesCommandFlags(t *testing.T) {
	flags := runCmd.Flags()
	defer func() {
		_ = flags.Set("xdebug", "false")
		_ = flags.Set("notices", "false")
	}()

	if err := flags.Parse([]string{"--xdebug", "wp", "plugin", "list", "--format=json", "--notices"}); err != nil {
		t.Fatal(err)
	}
	if xdebug, _ := flags.GetBool("xdebug"); !xdebug {
		t.Error("--xdebug before the command was not parsed as jcore's")
	}
	if notices, _ := flags.GetBool("notices"); notices {
		t.Error("--notices after the command was parsed as jcore's")
	}
	want := []string{"wp", "plugin", "list", "--format=json", "--notices"}
	if got := flags.Args(); !reflect.DeepEqual(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
}
