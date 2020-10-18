package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func fatalf(exit int, msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s: ", os.Args[0])
	fmt.Fprintf(os.Stderr, msg, args...)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(exit)
}

var root = &cobra.Command{
	Use:   "toyc <command>",
	Short: "toyc is a fast, lightweight toy container system.",
}

func main() {
	if err := root.Execute(); err != nil {
		fatalf(2, "%v", err)
	}
}
