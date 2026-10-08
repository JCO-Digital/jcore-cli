package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/JCO-Digital/jcore/internal/constants"
	"github.com/JCO-Digital/jcore/internal/proxy"
	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
)

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the status of the environment",
	Long:  `Verify that necessary folders exist, permissions are correct, and required external tools are installed.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking system status...")

		pass := true

		if !checkFolders() {
			pass = false
		}

		if !checkCommands() {
			pass = false
		}

		if !checkDocker() {
			pass = false
		}

		checkProxy()

		if pass {
			fmt.Println("\nEverything seems fine.")
		} else {
			fmt.Println("\nErrors encountered! Please check the output above.")
			os.Exit(1)
		}
	},
}

func checkFolders() bool {
	fmt.Println("\nChecking folders:")
	pass := true

	// Check Global Folders
	home, err := os.UserHomeDir()
	if err == nil {
		for _, folder := range constants.GlobalFolders {
			if !processFolder(filepath.Join(home, folder)) {
				pass = false
			}
		}
	}

	// Check Project Folders (if in project)
	curr, _ := os.Getwd()
	inProject := false
	for curr != "/" {
		if _, err := os.Stat(filepath.Join(curr, "jcore.toml")); err == nil {
			inProject = true
			break
		}
		curr = filepath.Dir(curr)
	}

	if inProject {
		for _, folder := range constants.ProjectFolders {
			if !processFolder(filepath.Join(curr, folder)) {
				pass = false
			}
		}
	}

	return pass
}

func processFolder(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		fmt.Printf("  [WAIT] Folder %s doesn't exist, creating it...\n", path)
		err = os.MkdirAll(path, 0755)
		if err != nil {
			fmt.Printf("  [FAIL] Failed to create folder %s: %v\n", path, err)
			return false
		}
		fmt.Printf("  [ OK ] Folder %s created.\n", path)
		return true
	}

	if !info.IsDir() {
		fmt.Printf("  [FAIL] %s exists but is not a directory.\n", path)
		return false
	}

	// Check write permission
	tmpFile := filepath.Join(path, ".jcore_doctor_test")
	f, err := os.Create(tmpFile)
	if err != nil {
		u, _ := user.Current()
		fmt.Printf("  [FAIL] Folder %s is not writable.\n", path)
		if runtime.GOOS != "windows" {
			fmt.Printf("         Fix this by running: sudo chown %s -R %s\n", u.Username, path)
		}
		return false
	}
	f.Close()
	os.Remove(tmpFile)

	fmt.Printf("  [ OK ] Folder %s OK.\n", path)
	return true
}

func checkCommands() bool {
	fmt.Println("\nChecking external commands:")
	pass := true

	for _, command := range constants.ExternalCommands {
		path, err := exec.LookPath(command.Name)
		if err != nil {
			fmt.Printf("  [FAIL] Command %s not found!\n", command.Name)
			pass = false
			continue
		}

		out, err := exec.Command(path, command.Version).CombinedOutput()
		if err != nil {
			fmt.Printf("  [FAIL] Command %s found at %s but failed to run: %v\n", command.Name, path, err)
			pass = false
			continue
		}

		// Just show the first line of version output
		versionLine := ""
		lines := fmt.Sprintf("%s", out)
		for _, line := range strings.Split(lines, "\n") {
			if strings.TrimSpace(line) != "" {
				versionLine = line
				break
			}
		}
		fmt.Printf("  [ OK ] %s found: %s\n", command.Name, versionLine)
	}

	return pass
}

func checkDocker() bool {
	fmt.Println("\nChecking Docker status:")
	_, err := exec.Command("docker", "info").CombinedOutput()
	if err != nil {
		fmt.Println("  [FAIL] Docker service test failed. Is Docker running?")
		return false
	}

	// Check if docker compose is available as a plugin (docker compose) or standalone (docker-compose)
	_, err1 := exec.LookPath("docker-compose")
	_, err2 := exec.Command("docker", "compose", "version").CombinedOutput()

	if err1 != nil && err2 != nil {
		fmt.Println("  [FAIL] Neither 'docker-compose' nor 'docker compose' plugin found.")
		return false
	}

	// The proxy override relies on compose's `!reset` tag.
	out, err := exec.Command("docker", "compose", "version", "--short").Output()
	if err != nil {
		fmt.Println("  [FAIL] Couldn't determine the Docker Compose version.")
		return false
	}
	composeVersion := strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
	current, err := version.NewVersion(composeVersion)
	if err != nil || current.LessThan(minComposeVersion) {
		fmt.Printf("  [FAIL] Docker Compose %s is too old, %s or newer is required.\n", composeVersion, minComposeVersion)
		return false
	}

	fmt.Printf("  [ OK ] Docker is running and Compose %s is available.\n", composeVersion)
	return true
}

// minComposeVersion is the oldest Docker Compose supporting the `!reset`
// tag the proxy override uses to drop a project's host ports.
var minComposeVersion = version.Must(version.NewVersion("2.24.0"))

// checkProxy reports the shared proxy's state, and whatever would keep it
// from starting. Neither is an error: `jcore start` starts the proxy, and
// offers to stop a project started by an older jcore holding its ports.
func checkProxy() {
	fmt.Println("\nChecking the JCore proxy:")
	running, err := proxy.Running()
	if err != nil {
		fmt.Printf("  [WARN] Couldn't check the proxy: %v\n", err)
		return
	}
	if running {
		fmt.Printf("  [ OK ] The proxy is running (dashboard %s).\n", proxy.DashboardURL)
		return
	}

	holders, err := proxy.PortHolders()
	if err != nil {
		fmt.Printf("  [WARN] Couldn't check ports 80/443: %v\n", err)
		return
	}
	if len(holders) == 0 {
		fmt.Println("  [ OK ] The proxy isn't running; ports 80/443 are free for \"jcore start\" to start it.")
		return
	}
	for _, h := range holders {
		fmt.Printf("  [WARN] The proxy isn't running and %s.\n", strings.Replace(h.String(), ":", " is taken by", 1))
	}
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
