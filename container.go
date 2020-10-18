package main

import (
	"context"
	"errors"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrContainerNotExist = errors.New("container does not exist")
)

// Container represents the properties of a container.
type Container struct {

	// Name is the name of this container.
	Name string

	// Root is the path to the root filesystem of this container.
	Root string

	// Argv is the argv array of the command executed by this container.
	Argv []string

	// Mounts is the list of mounts to perform at the container startup.
	Mounts []MountEntry

	// RuntimePath is the path to the currently running container.
	RuntimePath string
}

func (c *Container) Command(ctx context.Context, init bool) (*exec.Cmd, error) {

	var args []string

	// Init determines whether or not we are the init process of this
	// container. The init process always gets started with --persist.
	if init {
		args = append(args,
			"-r", c.Root,
			"--persist", c.RuntimePath)

		for _, mount := range c.Mounts {
			if mount.Target == "" {
				return nil, fmt.Errorf("Mount entry must have a non-empty target")
			}
			if mount.Source == "" {
				mount.Source = "none"
			}
			if mount.Type == "" {
				mount.Type = "none"
			}
			mountArg := fmt.Sprintf("source=%s,target=%s,type=%s,%s",
				mount.Source,
				mount.Target,
				mount.Type,
				strings.Join(mount.Options, ","))
			args = append(args, "--mount", mountArg)
		}
	} else {
		args = append(args, "--share", c.RuntimePath)
	}

	args = append(args, "--workdir", "/", "--")
	args = append(args, c.Argv...)

	return exec.CommandContext(ctx, "bst", args...), nil
}

// LoadContainerConfig loads a Container from the specified path.
func LoadContainerConfig(path string) (Container, error) {
	f, err := os.Open(filepath.Join(path, "container.json"))
	if os.IsNotExist(err) {
		return Container{}, ErrContainerNotExist
	}
	if err != nil {
		return Container{}, err
	}
	defer f.Close()

	var ctnr Container
	err = json.NewDecoder(f).Decode(&ctnr)

	if !filepath.IsAbs(ctnr.Root) {
		ctnr.Root = filepath.Join(path, ctnr.Root)
	}
	if !filepath.IsAbs(ctnr.RuntimePath) {
		ctnr.RuntimePath = filepath.Join(path, ctnr.RuntimePath)
	}
	return ctnr, err
}

// MountEntry represents a container mount operation.
type MountEntry struct {

	// Source is the mount source. If a path is specified, it is interpreted
	// relative to the host filesystem.
	Source string

	// Target is the mount target path. It is always interpreted relative to the
	// container root filesystem.
	Target string

	// Type is the mount type. Defaults to "none".
	Type string

	// Options is the list of mount options.
	Options []string
}
