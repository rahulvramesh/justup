package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/rahulvramesh/justup/pkg/kubernetes"
	"github.com/spf13/cobra"
)

var listAll bool

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all workspaces",
	Long: `List all workspaces in the cluster.

Examples:
  justup list
  justup ls
  justup list --all`,
	Run: runList,
}

func init() {
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false, "Include stopped workspaces")
}

func runList(cmd *cobra.Command, args []string) {
	client, err := kubernetes.NewClient()
	if err != nil {
		exitError("failed to create Kubernetes client", err)
	}

	ctx := context.Background()
	workspaces, err := client.ListWorkspaces(ctx, listAll)
	if err != nil {
		exitError("failed to list workspaces", err)
	}

	if len(workspaces) == 0 {
		fmt.Println("No workspaces found.")
		fmt.Println("\nCreate one with:")
		fmt.Println("  justup create github.com/user/repo")
		return
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tAGE\tGIT URL")
	for _, ws := range workspaces {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ws.Name, ws.Status, ws.Age, ws.GitURL)
	}
	w.Flush()
}
