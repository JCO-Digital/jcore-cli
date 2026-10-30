package docker

import (
	"reflect"
	"testing"
)

func TestComposeExecArgs(t *testing.T) {
	tests := []struct {
		name string
		opts ExecOptions
		want []string
	}{
		{
			name: "no tty",
			opts: ExecOptions{},
			want: []string{"compose", "exec", "-T", "wordpress", "wp", "option", "get", "siteurl"},
		},
		{
			name: "tty",
			opts: ExecOptions{TTY: true},
			want: []string{"compose", "exec", "wordpress", "wp", "option", "get", "siteurl"},
		},
		{
			name: "env in order, before the service",
			opts: ExecOptions{Env: []string{"XDEBUG_MODE=off", "SHELL_PIPE=0"}},
			want: []string{"compose", "exec", "-T", "-e", "XDEBUG_MODE=off", "-e", "SHELL_PIPE=0", "wordpress", "wp", "option", "get", "siteurl"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := composeExecArgs("wordpress", tt.opts, []string{"wp", "option", "get", "siteurl"})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("composeExecArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}
