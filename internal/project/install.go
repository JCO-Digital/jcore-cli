package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/JCO-Digital/jcore/internal/docker"
	"github.com/spf13/viper"
)

// InstallDependencies runs the project's host-side dependency installation
// (a Makefile's "install" target, or npm/pnpm plus Composer) and pulls
// updated Docker images, mirroring the legacy TypeScript CLI's
// finalizeProject(install, pull). It's a no-op unless the `install` setting
// is true or force is set (e.g. via `start --install`).
func InstallDependencies(projectDir string, force bool) error {
	if !viper.GetBool("install") && !force {
		return nil
	}

	if fileExists(projectDir, "Makefile") {
		fmt.Println("Makefile found, running 'make install'.")
		if err := runIn(projectDir, "make", "install"); err != nil {
			return fmt.Errorf("running make failed: %w", err)
		}
		return nil
	}

	if fileExists(projectDir, "package.json") {
		if err := installNodePackages(projectDir); err != nil {
			return err
		}
	}

	if fileExists(projectDir, "composer.json") {
		fmt.Println("Installing composer packages.")
		if err := runIn(projectDir, "composer", "install", "--quiet"); err != nil {
			return fmt.Errorf("composer failed, maybe not installed: %w", err)
		}
	}

	fmt.Println("Updating Docker images.")
	return docker.ComposePull(projectDir)
}

// installNodePackages installs npm packages, preferring npm's own lock file
// (via `npm ci`) if one exists and pnpm's doesn't, otherwise pnpm (`pnpm
// i`), enabling pnpm through corepack first if it isn't already available.
func installNodePackages(projectDir string) error {
	if err := runIn(projectDir, "pnpm", "--version"); err != nil {
		if err := runIn(projectDir, "corepack", "enable"); err != nil {
			return fmt.Errorf("running pnpm failed: %w", err)
		}
	}

	if fileExists(projectDir, "package-lock.json") && !fileExists(projectDir, "pnpm-lock.yaml") {
		fmt.Println("Installing npm packages from lock file.")
		if err := runIn(projectDir, "npm", "ci", "--silent", "--no-fund"); err != nil {
			return fmt.Errorf("running npm failed: %w", err)
		}
		return nil
	}

	fmt.Println("Installing pnpm packages.")
	if err := runIn(projectDir, "pnpm", "i"); err != nil {
		return fmt.Errorf("running pnpm failed: %w", err)
	}
	return nil
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func runIn(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
