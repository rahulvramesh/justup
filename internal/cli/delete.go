package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rahulvramesh/justup/pkg/kubernetes"
	"github.com/spf13/cobra"
)

var (
	deleteForce   bool
	deleteKeepPVC bool
)

var deleteCmd = &cobra.Command{
	Use:     "delete <workspace>",
	Aliases: []string{"rm"},
	Short:   "Delete a workspace",
	Long: `Delete a workspace and its associated resources.

By default, this will delete:
  - The workspace pod
  - The persistent volume claim (your code)
  - The SSH keys secret

Use --keep-pvc to preserve the persistent volume for later use.

Examples:
  justup delete myworkspace
  justup delete myworkspace --keep-pvc
  justup delete myworkspace --force`,
	Args: cobra.ExactArgs(1),
	Run:  runDelete,
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation")
	deleteCmd.Flags().BoolVar(&deleteKeepPVC, "keep-pvc", false, "Keep the persistent volume claim")
}

func runDelete(cmd *cobra.Command, args []string) {
	name := args[0]

	// Confirm deletion
	if !deleteForce {
		fmt.Printf("This will permanently delete workspace '%s'", name)
		if !deleteKeepPVC {
			fmt.Print(" and all its data")
		}
		fmt.Print(".\n")
		fmt.Print("Are you sure? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			return
		}
	}

	client, err := kubernetes.NewClient()
	if err != nil {
		exitError("failed to create Kubernetes client", err)
	}

	ctx := context.Background()
	opts := kubernetes.DeleteOptions{
		Name:    name,
		KeepPVC: deleteKeepPVC,
	}

	fmt.Printf("Deleting workspace '%s'...\n", name)
	if err := client.DeleteWorkspace(ctx, opts); err != nil {
		exitError("failed to delete workspace", err)
	}

	fmt.Println("Workspace deleted.")
}
