package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/rahulvramesh/justup/pkg/kubernetes"
	"github.com/spf13/cobra"
)

var startWait bool

var startCmd = &cobra.Command{
	Use:   "start <workspace>",
	Short: "Start a stopped workspace",
	Long: `Start a previously stopped workspace.

This will recreate the pod with the same configuration and attach the
existing persistent volume.

Examples:
  justup start myworkspace
  justup start myworkspace --wait`,
	Args: cobra.ExactArgs(1),
	Run:  runStart,
}

func init() {
	startCmd.Flags().BoolVarP(&startWait, "wait", "w", true, "Wait for workspace to be ready")
}

func runStart(cmd *cobra.Command, args []string) {
	name := args[0]

	client, err := kubernetes.NewClient()
	if err != nil {
		exitError("failed to create Kubernetes client", err)
	}

	ctx := context.Background()

	fmt.Printf("Starting workspace '%s'...\n", name)
	if err := client.StartWorkspace(ctx, name); err != nil {
		exitError("failed to start workspace", err)
	}

	if startWait {
		fmt.Print("Waiting for workspace to be ready")
		for i := 0; i < 60; i++ {
			ws, err := client.GetWorkspace(ctx, name)
			if err != nil {
				exitError("failed to get workspace status", err)
			}
			if ws.Status == "Running" {
				fmt.Println(" done!")
				fmt.Printf("\nTo connect:\n  justup ssh %s\n", name)
				return
			}
			fmt.Print(".")
			time.Sleep(2 * time.Second)
		}
		fmt.Println("\nWorkspace is still starting. Check status with: justup list")
	} else {
		fmt.Println("Workspace starting.")
	}
}
