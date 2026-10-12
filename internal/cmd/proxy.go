package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JCO-Digital/jcore/internal/proxy"
	"github.com/spf13/cobra"
)

// proxyCmd groups the commands managing the shared reverse proxy.
var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage the shared proxy that lets several projects run at once",
	Long: `The JCore proxy (Traefik) owns host ports 80/443 and routes each request to
the right running project by its domain, so several projects can run at the
same time. "jcore start" starts it automatically; it then keeps running
(also across reboots) until stopped with "jcore proxy stop".`,
}

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the proxy",
	Run: func(cmd *cobra.Command, args []string) {
		if err := ensureProxy(); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("The JCore proxy is running. Dashboard: %s\n", proxy.DashboardURL)
	},
}

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the proxy",
	Long: `Stops the proxy. Running projects keep running, but can't be reached from
the browser until the proxy is started again.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := proxy.Stop(); err != nil {
			fmt.Printf("Docker failed: %v\n", err)
		}
	},
}

var proxyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the proxy is running",
	Run: func(cmd *cobra.Command, args []string) {
		printProxyStatus()
	},
}

var proxyLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Follow the proxy's logs",
	Run: func(cmd *cobra.Command, args []string) {
		if err := proxy.Logs(); err != nil {
			fmt.Printf("Docker failed: %v\n", err)
		}
	},
}

// printProxyStatus prints one line describing the proxy's state.
func printProxyStatus() {
	running, err := proxy.Running()
	switch {
	case err != nil:
		fmt.Printf("Proxy: unknown (%v)\n", err)
	case running:
		fmt.Printf("Proxy: running (dashboard %s)\n", proxy.DashboardURL)
	default:
		fmt.Println("Proxy: stopped (started automatically by \"jcore start\")")
	}
}

// ensureProxy makes sure the shared proxy is running. If its ports are held
// by compose projects (typically ones started by an older jcore, which bind
// 80/443 themselves), it offers to stop them first.
func ensureProxy() error {
	err := proxy.Ensure()

	var conflict *proxy.PortConflictError
	if !errors.As(err, &conflict) {
		return err
	}

	for _, h := range conflict.Holders {
		if h.Project == "" {
			return fmt.Errorf("%v\nStop whatever else is listening on ports 80/443 (e.g. a local web server), then try again", conflict)
		}
	}

	projects := conflict.Projects()
	fmt.Println(conflict.Error())
	stop := false
	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Stop %s so the proxy can take over?", strings.Join(projects, ", ")),
		Default: true,
	}
	if err := survey.AskOne(prompt, &stop); err != nil || !stop {
		return errors.New("ports 80/443 are still in use, not starting")
	}

	for _, p := range projects {
		fmt.Printf("Stopping %s.\n", p)
		if err := proxy.StopComposeProject(p); err != nil {
			return fmt.Errorf("stopping %s: %w", p, err)
		}
	}
	return proxy.Ensure()
}

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.AddCommand(proxyStartCmd, proxyStopCmd, proxyStatusCmd, proxyLogsCmd)
}
