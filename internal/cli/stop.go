package cli

import (
	"context"
	"fmt"

	"github.com/rahulvramesh/justup/pkg/kubernetes"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <workspace>",
	Short: "Stop a running workspace",
	Long: `Stop a workspace to save resources. The persistent storage is preserved.

Use 'justup start' to resume the workspace later.

Examples:
  justup stop myworkspace`,
	Args: cobra.ExactArgs(1),
	Run:  runStop,
}

func runStop(cmd *cobra.Command, args []string) {
	name := args[0]

	client, err := kubernetes.NewClient()
	if err != nil {
		exitError("failed to create Kubernetes client", err)
	}

	ctx := context.Background()

	fmt.Printf("Stopping workspace '%s'...\n", name)
	if err := client.StopWorkspace(ctx, name); err != nil {
		exitError("failed to stop workspace", err)
	}

	fmt.Println("Workspace stopped. Data is preserved.")
	fmt.Printf("\nTo resume:\n  justup start %s\n", name)
}
