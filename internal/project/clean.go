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

// ListDockerProjects returns all JCore projects known to docker compose,
// i.e. compose projects whose directory contains a .jcore folder.
func ListDockerProjects() ([]DockerProject, error) {
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
		path := strings.TrimSuffix(strings.Split(p.ConfigFiles, ",")[0], "/docker-compose.yml")

		if _, err := os.Stat(filepath.Join(path, ".jcore")); err != nil {
			continue
		}

		projects = append(projects, DockerProject{
			Name:    p.Name,
			Path:    path,
			Running: strings.Contains(p.Status, "running"),
		})
	}

	return projects, nil
}

// CleanProject removes a project's containers and volumes, and deletes its .jcore workfiles.
func CleanProject(p DockerProject) error {
	fmt.Printf("Cleaning containers for %s.\n", p.Name)
	if err := docker.ComposeRm(p.Path); err != nil {
		return err
	}

	fmt.Printf("Cleaning volumes for project %s.\n", p.Name)
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

// CleanAll cleans every non-running JCore project, then prunes Docker globally.
func CleanAll() error {
	projects, err := ListDockerProjects()
	if err != nil {
		return err
	}

	for _, p := range projects {
		if !p.Running {
			if err := CleanProject(p); err != nil {
				fmt.Printf("Error cleaning %s: %v\n", p.Name, err)
			}
		}
	}

	return CleanDocker(true)
}

// CleanDocker prunes unused Docker containers, images, volumes, and networks.
func CleanDocker(all bool) error {
	fmt.Println("Cleaning Containers")
	if err := docker.PruneContainers(); err != nil {
		return err
	}

	fmt.Println("Cleaning Images")
	if err := docker.PruneImages(all); err != nil {
		return err
	}

	fmt.Println("Cleaning Volumes")
	if err := docker.PruneVolumes(all); err != nil {
		return err
	}

	fmt.Println("Cleaning Networks")
	return docker.PruneNetworks()
}
