package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/rahulvramesh/justup/pkg/database"
	"github.com/rahulvramesh/justup/pkg/kubernetes"
	"github.com/spf13/cobra"
)

var (
	createName    string
	createBranch  string
	createImage   string
	createCPU     string
	createMemory  string
	createStorage string
	createDinD    bool
)

var createCmd = &cobra.Command{
	Use:   "create <github-url>",
	Short: "Create a new workspace from a GitHub repository",
	Long: `Create a new development workspace in Kubernetes from a GitHub repository.

The workspace will be created with:
  - Debian-based container with SSH access
  - Persistent storage for your code
  - Optional Docker-in-Docker support

Examples:
  justup create github.com/user/repo
  justup create github.com/user/repo --name myproject
  justup create github.com/user/repo --name myproject --dind
  justup create https://github.com/user/repo --branch develop`,
	Args: cobra.ExactArgs(1),
	Run:  runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&createName, "name", "n", "", "Workspace name (defaults to repo name)")
	createCmd.Flags().StringVarP(&createBranch, "branch", "b", "main", "Git branch to clone")
	createCmd.Flags().StringVar(&createImage, "image", "ghcr.io/rahulvramesh/justup/devcontainer:latest", "Container image to use")
	createCmd.Flags().StringVar(&createCPU, "cpu", "1", "CPU limit")
	createCmd.Flags().StringVar(&createMemory, "memory", "2Gi", "Memory limit")
	createCmd.Flags().StringVar(&createStorage, "storage", "10Gi", "Persistent storage size")
	createCmd.Flags().BoolVar(&createDinD, "dind", false, "Enable Docker-in-Docker")
}

func runCreate(cmd *cobra.Command, args []string) {
	githubURL := normalizeGitHubURL(args[0])

	// Extract workspace name from repo if not provided
	if createName == "" {
		createName = extractRepoName(githubURL)
	}

	// Validate workspace name
	if !isValidWorkspaceName(createName) {
		exitError("invalid workspace name (must be lowercase alphanumeric with dashes)", nil)
	}

	fmt.Printf("Creating workspace '%s' from %s...\n", createName, githubURL)

	// Load SSH keys from database
	sshPubKeys := ""
	db, err := database.Open(getDBPath())
	if err == nil {
		defer db.Close()
		user, err := db.GetOrCreateDefaultUser()
		if err == nil {
			keys, err := db.ListSSHKeys(user.ID)
			if err == nil && len(keys) > 0 {
				var keyLines []string
				for _, key := range keys {
					keyLines = append(keyLines, key.PublicKey)
				}
				sshPubKeys = strings.Join(keyLines, "\n")
			}
		}
	}

	// Create Kubernetes client
	client, err := kubernetes.NewClient()
	if err != nil {
		exitError("failed to create Kubernetes client", err)
	}

	// Create workspace options
	opts := kubernetes.WorkspaceOptions{
		Name:       createName,
		GitURL:     githubURL,
		Branch:     createBranch,
		Image:      createImage,
		CPU:        createCPU,
		Memory:     createMemory,
		Storage:    createStorage,
		EnableDinD: createDinD,
		SSHPubKey:  sshPubKeys,
	}

	// Create the workspace
	ctx := context.Background()
	ws, err := client.CreateWorkspace(ctx, opts)
	if err != nil {
		exitError("failed to create workspace", err)
	}

	fmt.Printf("\nWorkspace created successfully!\n")
	fmt.Printf("  Name:   %s\n", ws.Name)
	fmt.Printf("  Status: %s\n", ws.Status)
	fmt.Printf("\nTo connect:\n")
	fmt.Printf("  justup ssh %s\n", ws.Name)
}

// normalizeGitHubURL ensures the URL is in a consistent format
func normalizeGitHubURL(url string) string {
	// Remove https:// or http:// prefix if present
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Add https:// prefix
	if !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	// Add .git suffix if not present
	if !strings.HasSuffix(url, ".git") {
		url = url + ".git"
	}

	return url
}

// extractRepoName gets the repository name from a GitHub URL
func extractRepoName(url string) string {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Get the last part of the path
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return strings.ToLower(parts[len(parts)-1])
	}
	return "workspace"
}

// isValidWorkspaceName checks if the name is valid for Kubernetes
func isValidWorkspaceName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	for i, c := range name {
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '-' && i > 0 && i < len(name)-1 {
			continue
		}
		return false
	}
	return true
}
