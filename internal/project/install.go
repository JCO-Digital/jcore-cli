package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/JCO-Digital/jcore/internal/docker"
	"github.com/JCO-Digital/jcore/internal/logging"
	"github.com/spf13/viper"
)

// composePull pulls the project's Docker images. Overridable by tests.
var composePull = docker.ComposePull

// ImagePullInterval is how often a plain start (with the install setting on)
// pulls the project's Docker images. Pulling on every start costs a few
// seconds and a network round trip even when nothing changed, while the
// images themselves are only rebuilt weekly. Images compose doesn't have
// yet are pulled by `docker compose up` regardless.
const ImagePullInterval = 24 * time.Hour

// imagePullStampRelPath records when the project's images were last pulled.
// It lives in .jcore, so `jcore clean` resets it.
const imagePullStampRelPath = ".jcore/images-pulled"

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
	} else {
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
	}

	return pullImages(projectDir, force)
}

// pullImages pulls the project's Docker images: always when forced (clone,
// init, `start --install`), otherwise at most once per ImagePullInterval. A
// scheduled pull that fails (e.g. offline) only warns, since the images
// already on disk still work; a forced one returns the error.
func pullImages(projectDir string, force bool) error {
	stamp := filepath.Join(projectDir, imagePullStampRelPath)
	if !force {
		if info, err := os.Stat(stamp); err == nil && time.Since(info.ModTime()) < ImagePullInterval {
			logging.Verbose("Docker images were updated less than %s ago, skipping the pull.", ImagePullInterval)
			return nil
		}
	}

	fmt.Println("Updating Docker images.")
	if err := composePull(projectDir); err != nil {
		if force {
			return err
		}
		logging.Warn("Updating Docker images failed, starting with the ones already on disk: %v", err)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(stamp), 0755); err == nil {
		_ = os.WriteFile(stamp, []byte(time.Now().Format(time.RFC3339)+"\n"), 0644)
	}
	return nil
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
