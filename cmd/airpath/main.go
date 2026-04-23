package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

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

func newStubCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Run:   func(cmd *cobra.Command, args []string) { fmt.Println("not yet implemented") },
	}
}
