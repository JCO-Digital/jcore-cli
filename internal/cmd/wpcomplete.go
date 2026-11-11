package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/JCO-Digital/jcore/internal/docker"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// wpCompletionsTimeout caps how long tab completion may wait on
// `wp cli completions` inside the wordpress container. Completions that
// hang (container down, wp boot stuck) should fail soft, not freeze the shell.
const wpCompletionsTimeout = 3 * time.Second

// wpCLICompletions runs `wp cli completions` in the project's wordpress
// container. Xdebug is off so completions stay fast and its stderr chatter
// never reaches the shell. Tests replace this to avoid docker.
var wpCLICompletions = func(projectDir, line string, point int) (string, error) {
	return docker.ComposeExecStdoutTimeout(projectDir, "wordpress", docker.ExecOptions{
		Env: []string{"XDEBUG_MODE=off"},
	}, []string{
		"wp", "cli", "completions",
		"--line=" + line,
		fmt.Sprintf("--point=%d", point),
	}, wpCompletionsTimeout)
}

// projectRootForCompletion looks up the current project. Tests replace it.
var projectRootForCompletion = project.FindProjectRoot

// completeRunArgs completes `jcore run` arguments. Once the command is
// `wp`, further words come from wp-cli inside the wordpress container so
// the suggestions match the site's installed commands, plugins and themes.
func completeRunArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		if strings.HasPrefix("wp", toComplete) {
			return []string{"wp"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveDefault
	}
	if args[0] != "wp" {
		return nil, cobra.ShellCompDirectiveDefault
	}

	projectDir, err := projectRootForCompletion()
	if err != nil || projectDir == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	line := wpCompletionLine(args, toComplete)
	out, err := wpCLICompletions(projectDir, line, len(line))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return parseWPCompletions(out)
}

// wpCompletionLine builds the COMP_LINE wp-cli expects from jcore run's
// already-completed args (which start with "wp") and the partial word.
func wpCompletionLine(args []string, toComplete string) string {
	parts := append(append([]string{}, args...), toComplete)
	return strings.Join(parts, " ")
}

// parseWPCompletions turns `wp cli completions` output into cobra
// suggestions. An empty result or a <file> token means fall back to file
// completion, matching wp-cli's own bash helper. Candidates keep any
// trailing spaces wp-cli includes (with NoSpace, those spaces are how a
// finished word is separated from the next).
func parseWPCompletions(out string) ([]string, cobra.ShellCompDirective) {
	out = strings.ReplaceAll(out, "\r\n", "\n")
	out = strings.TrimSuffix(out, "\n")
	if out == "" {
		return nil, cobra.ShellCompDirectiveDefault
	}

	var completions []string
	fileComp := false
	for _, line := range strings.Split(out, "\n") {
		if line == "<file>" || strings.HasPrefix(line, "<file>") {
			fileComp = true
			continue
		}
		completions = append(completions, line)
	}
	if fileComp || len(completions) == 0 {
		return completions, cobra.ShellCompDirectiveDefault
	}
	return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}
