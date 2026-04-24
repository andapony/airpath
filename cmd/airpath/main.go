package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// main builds the root Cobra command, registers sub-commands, and delegates to
// cobra.Command.Execute, which parses os.Args and dispatches the appropriate
// sub-command. Any error from Execute is printed to stderr and exits with code 1.
func main() {
	root := &cobra.Command{
		Use:   "airpath",
		Short: "Generate impulse response WAV files for simulated acoustic spaces",
	}
	root.AddCommand(newGenerateCmd())
	root.AddCommand(newStubCmd("info", "Print room analysis (RT60, modes, path counts)"))
	root.AddCommand(newStubCmd("materials", "List available materials and absorption coefficients"))
	root.AddCommand(newStubCmd("validate", "Validate a scene file without generating output"))
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newStubCmd returns a placeholder Cobra command that prints "not yet
// implemented" and exits cleanly. Use this for commands planned in future
// milestones so they appear in --help without being functional yet.
func newStubCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Run:   func(cmd *cobra.Command, args []string) { fmt.Println("not yet implemented") },
	}
}
