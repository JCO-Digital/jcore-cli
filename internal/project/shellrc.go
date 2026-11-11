package project

import (
	"os"
	"path/filepath"
)

// Bash rcfile for `jcore shell`. Written into the project and passed as
// bash --rcfile, so the interactive shell still gets the container user's
// normal bashrc and also has wp-cli tab completion. The project root is
// mounted at /project in the wordpress container.
const (
	shellRCRelPath       = ".jcore/shell.rc"
	shellRCContainerPath = "/project/" + shellRCRelPath
)

// shellRCContent is the rcfile `jcore shell` starts bash with. It sources
// the container's usual interactive bash startup files (bash --rcfile
// skips them), then registers the same wp-cli completion function as
// wp-cli's utils/wp-completion.bash.
const shellRCContent = `# Written by jcore shell. Rewritten when its contents change.
# bash --rcfile skips the usual startup files, so load them first.
[ -f /etc/bash.bashrc ] && . /etc/bash.bashrc
[ -f ~/.bashrc ] && . ~/.bashrc

# bash completion for the ` + "`wp`" + ` command (wp-cli utils/wp-completion.bash)
_wp_complete() {
	local OLD_IFS="$IFS"
	local cur=${COMP_WORDS[COMP_CWORD]}

	IFS=$'\n';  # want to preserve spaces at the end
	local opts
	opts="$(wp cli completions --line="$COMP_LINE" --point="$COMP_POINT" 2>/dev/null)"

	if [[ "$opts" =~ \<file\>[[:space:]]* ]]
	then
		COMPREPLY=( $(compgen -f -- "$cur") )
	elif [[ -z "$opts" ]]
	then
		COMPREPLY=( $(compgen -f -- "$cur") )
	else
		COMPREPLY=( ${opts} )
	fi

	IFS="$OLD_IFS"
	return 0
}
complete -o nospace -F _wp_complete wp
`

// WriteShellRC writes the bash rcfile `jcore shell` uses into projectDir,
// leaving it untouched when already up to date, and returns the path of
// that file inside the wordpress container.
func WriteShellRC(projectDir string) (containerPath string, err error) {
	path := filepath.Join(projectDir, shellRCRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := writeIfChanged(path, []byte(shellRCContent)); err != nil {
		return "", err
	}
	return shellRCContainerPath, nil
}
