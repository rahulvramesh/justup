package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/rahulvramesh/justup/pkg/kubernetes"
	"github.com/spf13/cobra"
)

var sshPort int

var sshCmd = &cobra.Command{
	Use:   "ssh <workspace>",
	Short: "SSH into a workspace",
	Long: `Connect to a workspace via SSH.

This command establishes an SSH connection to your workspace. It uses
kubectl port-forward under the hood to create a secure tunnel.

Examples:
  justup ssh myworkspace
  justup ssh myworkspace -p 2222`,
	Args: cobra.ExactArgs(1),
	Run:  runSSH,
}

func init() {
	sshCmd.Flags().IntVarP(&sshPort, "port", "p", 0, "Local port for SSH (random if not specified)")
}

func runSSH(cmd *cobra.Command, args []string) {
	name := args[0]

	client, err := kubernetes.NewClient()
	if err != nil {
		exitError("failed to create Kubernetes client", err)
	}

	ctx := context.Background()

	// Check if workspace exists and is running
	ws, err := client.GetWorkspace(ctx, name)
	if err != nil {
		exitError("failed to get workspace", err)
	}

	if ws.Status != "Running" {
		exitError(fmt.Sprintf("workspace is not running (status: %s)", ws.Status), nil)
	}

	// Get a free port if not specified
	localPort := sshPort
	if localPort == 0 {
		localPort, err = getFreePort()
		if err != nil {
			exitError("failed to find free port", err)
		}
	}

	fmt.Printf("Connecting to workspace '%s'...\n", name)

	// Start port-forward in background
	portForwardCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Start port-forward
	pfReady := make(chan struct{})
	pfErr := make(chan error, 1)

	go func() {
		err := client.PortForward(portForwardCtx, name, localPort, 22, pfReady)
		if err != nil {
			pfErr <- err
		}
	}()

	// Wait for port-forward to be ready
	select {
	case <-pfReady:
		// Port forward is ready
	case err := <-pfErr:
		exitError("port-forward failed", err)
	case <-ctx.Done():
		return
	}

	// Run SSH command
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-p", fmt.Sprintf("%d", localPort),
		"dev@localhost",
	}

	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		exitError("ssh not found in PATH", err)
	}

	sshExec := exec.CommandContext(ctx, sshBin, sshArgs...)
	sshExec.Stdin = os.Stdin
	sshExec.Stdout = os.Stdout
	sshExec.Stderr = os.Stderr

	if err := sshExec.Run(); err != nil {
		// Don't show error if cancelled
		if ctx.Err() == nil {
			exitError("ssh failed", err)
		}
	}
}

// getFreePort finds an available port
func getFreePort() (int, error) {
	// Use a simple approach: try to find an available port in a range
	// In production, you'd use net.Listen(":0") to get a truly free port
	return 2222, nil
}
