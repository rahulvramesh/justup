package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/rahulvramesh/justup/pkg/kubernetes"
	"github.com/spf13/cobra"
)

var ideCmd = &cobra.Command{
	Use:   "ide",
	Short: "Open workspace in an IDE",
	Long: `Open a workspace in your preferred IDE.

Supports VS Code Remote-SSH and JetBrains Gateway.`,
}

var ideVSCodeCmd = &cobra.Command{
	Use:   "vscode <workspace>",
	Short: "Open workspace in VS Code",
	Long: `Open a workspace in Visual Studio Code using Remote-SSH.

This will open VS Code and connect to your workspace via SSH.
Requires VS Code with the Remote-SSH extension installed.

Examples:
  justup ide vscode myworkspace`,
	Args: cobra.ExactArgs(1),
	Run:  runIDEVSCode,
}

var ideJetBrainsCmd = &cobra.Command{
	Use:   "jetbrains <workspace>",
	Short: "Open workspace with JetBrains Gateway",
	Long: `Open a workspace with JetBrains Gateway.

This will generate the connection URL for JetBrains Gateway.
Requires JetBrains Gateway installed.

Examples:
  justup ide jetbrains myworkspace`,
	Args: cobra.ExactArgs(1),
	Run:  runIDEJetBrains,
}

var proxyHost string

func init() {
	ideCmd.AddCommand(ideVSCodeCmd)
	ideCmd.AddCommand(ideJetBrainsCmd)

	ideCmd.PersistentFlags().StringVar(&proxyHost, "proxy", "", "SSH proxy host (e.g., proxy.justup.local)")

	rootCmd.AddCommand(ideCmd)
}

func runIDEVSCode(cmd *cobra.Command, args []string) {
	workspaceName := args[0]

	// Verify workspace exists and is running
	client, err := kubernetes.NewClient()
	if err != nil {
		exitError("failed to create Kubernetes client", err)
	}

	ctx := context.Background()
	ws, err := client.GetWorkspace(ctx, workspaceName)
	if err != nil {
		exitError("failed to get workspace", err)
	}

	if ws.Status != "Running" {
		exitError(fmt.Sprintf("workspace is not running (status: %s)", ws.Status), nil)
	}

	// Build the SSH connection string
	var sshTarget string
	if proxyHost != "" {
		// Use SSH proxy
		sshTarget = fmt.Sprintf("%s@%s", workspaceName, proxyHost)
	} else {
		// Use direct connection (requires port-forward or in-cluster access)
		fmt.Println("Note: No --proxy specified. Using direct connection.")
		fmt.Println("For remote access, use: justup ide vscode myworkspace --proxy proxy.justup.local")
		sshTarget = fmt.Sprintf("dev@%s", ws.PodIP)
	}

	// Build VS Code URL
	// Format: vscode://vscode-remote/ssh-remote+<ssh-target>/home/dev/workspace
	vscodeURL := fmt.Sprintf("vscode://vscode-remote/ssh-remote+%s/home/dev/workspace", sshTarget)

	fmt.Printf("Opening VS Code for workspace '%s'...\n", workspaceName)
	fmt.Printf("SSH target: %s\n", sshTarget)

	// Open VS Code
	if err := openURL(vscodeURL); err != nil {
		// Fallback: print the URL for manual opening
		fmt.Printf("\nCouldn't open VS Code automatically.\n")
		fmt.Printf("Open this URL manually: %s\n", vscodeURL)
		fmt.Printf("\nOr run: code --remote ssh-remote+%s /home/dev/workspace\n", sshTarget)
		return
	}

	fmt.Println("VS Code should open shortly.")
}

func runIDEJetBrains(cmd *cobra.Command, args []string) {
	workspaceName := args[0]

	// Verify workspace exists and is running
	client, err := kubernetes.NewClient()
	if err != nil {
		exitError("failed to create Kubernetes client", err)
	}

	ctx := context.Background()
	ws, err := client.GetWorkspace(ctx, workspaceName)
	if err != nil {
		exitError("failed to get workspace", err)
	}

	if ws.Status != "Running" {
		exitError(fmt.Sprintf("workspace is not running (status: %s)", ws.Status), nil)
	}

	// Build SSH connection info
	var sshHost string
	if proxyHost != "" {
		sshHost = proxyHost
	} else {
		sshHost = ws.PodIP
	}

	sshUser := "dev"
	if proxyHost != "" {
		sshUser = workspaceName // Use workspace name as username for proxy
	}

	fmt.Printf("JetBrains Gateway connection info for '%s':\n\n", workspaceName)
	fmt.Printf("  Host: %s\n", sshHost)
	fmt.Printf("  User: %s\n", sshUser)
	fmt.Printf("  Port: 22\n")
	fmt.Printf("  Project path: /home/dev/workspace\n")
	fmt.Printf("\nOpen JetBrains Gateway and create a new SSH connection with these details.\n")
}

// openURL opens a URL in the default browser/application
func openURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		// Try xdg-open first, then fallback to other options
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", url)
		} else if _, err := exec.LookPath("gnome-open"); err == nil {
			cmd = exec.Command("gnome-open", url)
		} else {
			return fmt.Errorf("no URL opener found")
		}
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
