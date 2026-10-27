package project

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JCO-Digital/jcore/internal/docker"
)

// DockerProject describes a JCore project known to docker compose.
type DockerProject struct {
	Name    string
	Path    string
	Running bool
}

type dockerComposeListing struct {
	Name        string `json:"Name"`
	Status      string `json:"Status"`
	ConfigFiles string `json:"ConfigFiles"`
}

// listComposeProjects returns every project known to docker compose,
// JCore or not, with the directory of its (first) compose file.
func listComposeProjects() ([]DockerProject, error) {
	out, err := exec.Command("docker", "compose", "ls", "-a", "--format", "json").Output()
	if err != nil {
		return nil, err
	}

	var raw []dockerComposeListing
	if err := json.Unmarshal(out, &raw); err != nil {
		var single dockerComposeListing
		if err := json.Unmarshal(out, &single); err != nil {
			return nil, err
		}
		raw = []dockerComposeListing{single}
	}

	var projects []DockerProject
	for _, p := range raw {
		projects = append(projects, DockerProject{
			Name:    p.Name,
			Path:    filepath.Dir(strings.Split(p.ConfigFiles, ",")[0]),
			Running: strings.Contains(p.Status, "running"),
		})
	}

	return projects, nil
}

// ListDockerProjects returns all JCore projects known to docker compose,
// i.e. compose projects whose directory contains a .jcore folder.
func ListDockerProjects() ([]DockerProject, error) {
	all, err := listComposeProjects()
	if err != nil {
		return nil, err
	}

	var projects []DockerProject
	for _, p := range all {
		if _, err := os.Stat(filepath.Join(p.Path, ".jcore")); err != nil {
			continue
		}
		projects = append(projects, p)
	}

	return projects, nil
}

// CleanProject stops and removes a project's containers and volumes, and
// deletes its .jcore workfiles. p.Name must be the compose project name
// (see ComposeProjectName), which is what Docker labels the volumes with.
func CleanProject(p DockerProject) error {
	fmt.Printf("Removing containers and volumes for %s.\n", p.Name)
	if err := docker.ComposeDown(p.Path); err != nil {
		return err
	}

	// down only removes volumes the compose files still declare; catch any
	// left over from an older compose file.
	volumes, err := docker.VolumesByProject(p.Name)
	if err != nil {
		return err
	}
	for _, v := range volumes {
		if err := docker.VolumeRm(v); err != nil {
			return err
		}
	}

	return os.RemoveAll(filepath.Join(p.Path, ".jcore"))
}

// StoppedProjects returns the JCore projects that aren't running: the ones
// `jcore clean all` removes.
func StoppedProjects() ([]DockerProject, error) {
	projects, err := ListDockerProjects()
	if err != nil {
		return nil, err
	}

	var stopped []DockerProject
	for _, p := range projects {
		if !p.Running {
			stopped = append(stopped, p)
		}
	}
	return stopped, nil
}

// CleanProjects cleans each of projects (see CleanProject), then removes
// dangling images. It deliberately prunes nothing else machine-wide: that
// would also delete volumes and images belonging to other Docker work.
func CleanProjects(projects []DockerProject) error {
	for _, p := range projects {
		if err := CleanProject(p); err != nil {
			fmt.Printf("Error cleaning %s: %v\n", p.Name, err)
		}
	}

	fmt.Println("Removing dangling images.")
	return docker.PruneImages(false)
}

// CleanDocker prunes Docker machine-wide, not just JCore: stopped
// containers, dangling images, anonymous volumes and unused networks.
// Named volumes (databases) and tagged images are kept.
func CleanDocker() error {
	fmt.Println("Cleaning Containers")
	if err := docker.PruneContainers(); err != nil {
		return err
	}

	fmt.Println("Cleaning Images")
	if err := docker.PruneImages(false); err != nil {
		return err
	}

	fmt.Println("Cleaning Volumes")
	if err := docker.PruneVolumes(false); err != nil {
		return err
	}

	fmt.Println("Cleaning Networks")
	return docker.PruneNetworks()
}
