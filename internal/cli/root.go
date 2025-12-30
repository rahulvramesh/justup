package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version information (set via ldflags)
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "justup",
	Short: "Kubernetes-based development environments",
	Long: `Justup is a CLI tool for creating and managing development environments
in Kubernetes. Create a dev environment from any GitHub repository with
SSH access, persistent storage, and Docker-in-Docker support.

Example:
  justup create github.com/user/repo --name myworkspace
  justup ssh myworkspace
  justup list
  justup delete myworkspace`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(startCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("justup version %s\n", Version)
		fmt.Printf("  Git commit: %s\n", GitCommit)
		fmt.Printf("  Build date: %s\n", BuildDate)
	},
}

// Helper to print errors consistently
func exitError(msg string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s: %v\n", msg, err)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	}
	os.Exit(1)
}
