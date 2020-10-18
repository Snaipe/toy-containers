package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

func init() {
	cmd := cobra.Command{
		Use:   "exec [options] [--] <name> <program> [args...]",
		Short: "execute a program in the named container.",
		Run:   execCmd,
		Args:  cobra.MinimumNArgs(2),
	}
	// Disable flag parsing after the first non-flag argument. This allows us
	// to type commands like `toyc exec ls -l` instead of `toyc exec -- ls -l`.
	cmd.Flags().SetInterspersed(false)
	root.AddCommand(&cmd)
}

func execCmd(_ *cobra.Command, args []string) {

	var (
		name = args[0]
		argv = args[1:]
	)

	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home := os.Getenv("HOME")
		if home == "" {
			fatalf(1, "no state home configured -- set the XDG_STATE_HOME " +
					"or HOME environment variable.")
		}
	}

	path := filepath.Join(stateHome, "toyc", "containers", name)

	ctnr, err := LoadContainerConfig(path)
	if err != nil {
		fatalf(1, "exec %s %s: loading container: %v", name, strings.Join(argv, " "), err)
	}
	ctnr.Argv = argv

	// If the runtime path does not exist, we are the first process to start
	// the container, and its init process.
	init := false
	if _, err := os.Stat(ctnr.RuntimePath); os.IsNotExist(err) {
		if err := os.MkdirAll(ctnr.RuntimePath, 0777); err != nil {
			fatalf(1, "exec %s: %v", name, err)
		}
		init = true
	} else if err != nil {
		fatalf(1, "exec %s: %v", name, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	cmd, err := ctnr.Command(ctx, init)
	if err != nil {
		fatalf(1, "exec %s %s: preparing command: %v", name, strings.Join(argv, " "), err)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = []string{
		"TERM=" + os.Getenv("TERM"),
		"PATH=/bin:/usr/bin:/sbin:/usr/sbin",
	}

	done := make(chan error, 1)

	go func() {
		done <- cmd.Run()
		close(done)
	}()

	select {
	case <-signals:
		cancel()
	case err = <-done:
	}
	// Make sure the command has been waited for.
	<-done

	if init {
		// If we are init, the pid namespace dies with us. Cleanup the runtime
		// directory on exit.
		unpersist := exec.Command("bst-unpersist", ctnr.RuntimePath)
		unpersist.Stdout = os.Stdout
		unpersist.Stderr = os.Stderr
		if err := unpersist.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup: bst-unpersist %s: %v", ctnr.RuntimePath, err)
		}
		if err := os.Remove(ctnr.RuntimePath); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup: rmdir %s: %v", ctnr.RuntimePath, err)
		}
	}

	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			// Propagate a sensible exit status
			status := err.Sys().(syscall.WaitStatus)
			switch {
			case status.Exited():
				os.Exit(status.ExitStatus())
			case status.Signaled():
				os.Exit(128 + int(status.Signal()))
			}
		}
		fatalf(1, "exec %s %s: running command: %v", name, strings.Join(argv, " "), err)
	}
}
