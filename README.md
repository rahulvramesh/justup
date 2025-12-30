# Justup

**Kubernetes-based Development Environments CLI**

Justup is a CLI tool for creating and managing development environments in Kubernetes. Spin up a dev environment from any GitHub repository with SSH access, persistent storage, and Docker-in-Docker support.

Inspired by [Coder](https://coder.com) and [DevPod](https://devpod.sh).

---

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands Reference](#commands-reference)
- [How It Works](#how-it-works)
- [SSH Access](#ssh-access)
- [IDE Integration](#ide-integration)
- [Configuration](#configuration)
- [Kubernetes Resources](#kubernetes-resources)
- [Development](#development)
- [Troubleshooting](#troubleshooting)

---

## Features

- **One-Command Setup**: Create a dev environment from any GitHub URL
- **SSH Access**: Connect via standard SSH client or through the SSH proxy
- **Persistent Storage**: Your code persists across pod restarts via PVCs
- **Docker-in-Docker**: Optional DinD sidecar for container workflows
- **IDE Integration**: Native support for VS Code Remote-SSH and JetBrains Gateway
- **Resource Management**: Start, stop, and delete workspaces on demand
- **SSH Key Management**: Register and manage SSH public keys for authentication

---

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER'S MACHINE                                  │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  justup CLI                                                          │   │
│  │  • Create/manage workspaces                                          │   │
│  │  • SSH key management                                                │   │
│  │  • IDE integration (VS Code, JetBrains)                              │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                    │                              │                          │
│                    │ kubectl API                  │ SSH (port 22)            │
│                    ▼                              ▼                          │
└─────────────────────────────────────────────────────────────────────────────┘
                     │                              │
                     ▼                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           KUBERNETES CLUSTER                                 │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  justup-system namespace                                             │   │
│  │  ┌─────────────────────┐    ┌─────────────────────┐                 │   │
│  │  │   SSH Proxy         │    │   SQLite DB         │                 │   │
│  │  │   (Deployment)      │    │   (PVC)             │                 │   │
│  │  │   • Auth via pubkey │    │   • SSH keys        │                 │   │
│  │  │   • Route to pods   │    │   • Workspace meta  │                 │   │
│  │  │   • TCP proxying    │    │                     │                 │   │
│  │  └─────────────────────┘    └─────────────────────┘                 │   │
│  │            │                                                         │   │
│  │            │ LoadBalancer/NodePort (port 22)                        │   │
│  └────────────┼────────────────────────────────────────────────────────┘   │
│               │                                                             │
│               ▼                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  justup-workspaces namespace                                         │   │
│  │                                                                      │   │
│  │  ┌─────────────────────────────────────────────────────────────┐    │   │
│  │  │  Workspace Pod (ws-myproject)                                │    │   │
│  │  │  ┌───────────────────┐  ┌───────────────────┐               │    │   │
│  │  │  │  workspace        │  │  dind (optional)  │               │    │   │
│  │  │  │  container        │  │  container        │               │    │   │
│  │  │  │  • Debian         │  │  • Docker daemon  │               │    │   │
│  │  │  │  • SSH server:22  │  │  • Privileged     │               │    │   │
│  │  │  │  • Dev tools      │◄─┤  • Socket shared  │               │    │   │
│  │  │  │  • Git, curl...   │  │                   │               │    │   │
│  │  │  └───────────────────┘  └───────────────────┘               │    │   │
│  │  │           │                                                  │    │   │
│  │  │           ▼                                                  │    │   │
│  │  │  ┌───────────────────┐  ┌───────────────────┐               │    │   │
│  │  │  │  PVC              │  │  Secret           │               │    │   │
│  │  │  │  /home/dev/       │  │  SSH authorized   │               │    │   │
│  │  │  │  workspace        │  │  keys             │               │    │   │
│  │  │  └───────────────────┘  └───────────────────┘               │    │   │
│  │  └─────────────────────────────────────────────────────────────┘    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Component Description

| Component | Description |
|-----------|-------------|
| **justup CLI** | Command-line tool for managing workspaces. Communicates with Kubernetes API. |
| **SSH Proxy** | Routes SSH connections to workspace pods based on username. Handles authentication. |
| **Workspace Pod** | Development container with SSH server, dev tools, and optional Docker-in-Docker. |
| **PVC** | Persistent storage for workspace data. Survives pod restarts. |
| **Secret** | Stores SSH authorized_keys for workspace authentication. |
| **SQLite DB** | Stores SSH public keys and workspace metadata. |

---

## Installation

### Prerequisites

- Go 1.22+ (for building from source)
- Docker (for building container images)
- kubectl configured with cluster access
- A Kubernetes cluster (local or remote)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/rahulvramesh/justup.git
cd justup

# Download dependencies
make deps

# Build the CLI
make build

# (Optional) Install to PATH
make install-user  # Installs to ~/.local/bin
# or
make install       # Installs to /usr/local/bin (requires sudo)
```

### Build Docker Images

```bash
# Build the development container image
make docker-build-devcontainer

# Build the SSH proxy image
make docker-build-sshproxy

# Push to registry (configure DOCKER_REGISTRY first)
export DOCKER_REGISTRY=yourusername
make docker-push
```

### Deploy to Kubernetes

```bash
# Create namespaces and RBAC
make k8s-setup

# Deploy SSH proxy (optional, for remote SSH access)
make k8s-deploy-proxy

# Or do everything at once
make k8s-deploy
```

---

## Quick Start

```bash
# 1. Register your SSH key
justup ssh-key add ~/.ssh/id_ed25519.pub

# 2. Create a workspace from a GitHub repository
justup create github.com/expressjs/express --name my-express --branch master

# 3. Wait for the workspace to be ready
justup list

# 4. Connect via SSH
justup ssh my-express

# 5. Or open in VS Code
justup ide vscode my-express

# 6. When done, stop the workspace (preserves data)
justup stop my-express

# 7. Later, resume work
justup start my-express

# 8. Delete when finished
justup delete my-express
```

---

## Commands Reference

### Workspace Management

#### `justup create <github-url>`

Create a new workspace from a GitHub repository.

```bash
justup create github.com/user/repo
justup create github.com/user/repo --name myproject
justup create github.com/user/repo --name myproject --branch develop
justup create github.com/user/repo --name myproject --dind  # Enable Docker-in-Docker
justup create github.com/user/repo --cpu 2 --memory 4Gi --storage 20Gi
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--name, -n` | repo name | Workspace name |
| `--branch, -b` | main | Git branch to clone |
| `--image` | justup/devcontainer:latest | Container image |
| `--cpu` | 1 | CPU limit |
| `--memory` | 2Gi | Memory limit |
| `--storage` | 10Gi | PVC storage size |
| `--dind` | false | Enable Docker-in-Docker |

#### `justup list`

List all workspaces.

```bash
justup list
justup ls          # Alias
justup list --all  # Include stopped workspaces
```

**Output:**
```
NAME          STATUS   AGE   GIT URL
my-express    Running  2h    https://github.com/expressjs/express.git
my-project    Pending  5m    https://github.com/user/project.git
```

#### `justup delete <workspace>`

Delete a workspace.

```bash
justup delete myworkspace
justup delete myworkspace --force     # Skip confirmation
justup delete myworkspace --keep-pvc  # Keep persistent storage
```

#### `justup stop <workspace>`

Stop a workspace (delete pod, keep PVC).

```bash
justup stop myworkspace
```

#### `justup start <workspace>`

Start a previously stopped workspace.

```bash
justup start myworkspace
justup start myworkspace --wait=false  # Don't wait for ready
```

### SSH Connection

#### `justup ssh <workspace>`

Connect to a workspace via SSH.

```bash
justup ssh myworkspace
justup ssh myworkspace -p 2222  # Custom local port
```

**How it works:**
1. Verifies workspace exists and is running
2. Creates a port-forward from localhost to pod:22
3. Executes `ssh dev@localhost:<port>`

### SSH Key Management

#### `justup ssh-key add <path>`

Register an SSH public key.

```bash
justup ssh-key add ~/.ssh/id_ed25519.pub
justup ssh-key add ~/.ssh/id_rsa.pub --name "Work Laptop"
```

#### `justup ssh-key list`

List registered SSH keys.

```bash
justup ssh-key list
```

**Output:**
```
NAME         FINGERPRINT              ADDED      LAST USED
my-laptop    SHA256:abc123...         2d ago     1h ago
work-key     SHA256:def456...         5d ago     never
```

#### `justup ssh-key remove <fingerprint>`

Remove an SSH key.

```bash
justup ssh-key remove SHA256:abc123
```

### IDE Integration

#### `justup ide vscode <workspace>`

Open workspace in VS Code with Remote-SSH.

```bash
justup ide vscode myworkspace
justup ide vscode myworkspace --proxy proxy.example.com
```

#### `justup ide jetbrains <workspace>`

Get JetBrains Gateway connection info.

```bash
justup ide jetbrains myworkspace
```

### Other Commands

#### `justup version`

Print version information.

```bash
justup version
```

---

## How It Works

### Workspace Creation Flow

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    WORKSPACE CREATION FLOW                                │
└──────────────────────────────────────────────────────────────────────────┘

User runs: justup create github.com/user/repo --name myproject

    │
    ▼
┌─────────────────────────────────────────────┐
│ 1. Parse GitHub URL                         │
│    • Normalize URL format                   │
│    • Extract repo name if --name not given  │
│    • Validate workspace name (DNS-safe)     │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 2. Create Kubernetes Resources              │
│    • PersistentVolumeClaim (storage)        │
│    • Secret (SSH authorized_keys)           │
│    • Pod (workspace container)              │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 3. Init Container: git-clone                │
│    • Runs alpine/git image                  │
│    • Clones repo to /workspace              │
│    • Skips if .git already exists           │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 4. Main Container: workspace                │
│    • Runs justup/devcontainer image         │
│    • Starts SSH server on port 22           │
│    • Mounts PVC at /home/dev/workspace      │
│    • Mounts SSH keys at /home/dev/.ssh      │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 5. (Optional) Sidecar: dind                 │
│    • Runs docker:24-dind image              │
│    • Privileged container                   │
│    • Shares /var/run/docker.sock            │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 6. Ready for Connection                     │
│    • Pod status: Running                    │
│    • SSH server listening on :22            │
│    • User can connect via justup ssh        │
└─────────────────────────────────────────────┘
```

### Pod Lifecycle States

```
                    ┌──────────┐
                    │ Pending  │ ◄── Pod created, waiting for scheduling
                    └────┬─────┘
                         │
                         ▼
                    ┌──────────┐
                    │  Init    │ ◄── Init container cloning repo
                    └────┬─────┘
                         │
           ┌─────────────┴─────────────┐
           │                           │
           ▼                           ▼
    ┌──────────────┐           ┌──────────────┐
    │   Running    │           │  Init:Error  │ ◄── Clone failed
    └──────┬───────┘           └──────────────┘
           │
    ┌──────┴───────┐
    │              │
    ▼              ▼
┌────────┐   ┌───────────┐
│ Stopped│   │ Failed    │
│ (PVC   │   │ (Crash/   │
│ exists)│   │  OOM)     │
└────────┘   └───────────┘
```

---

## SSH Access

### Method 1: Direct via CLI (Port Forward)

The simplest method - uses kubectl port-forward under the hood.

```bash
justup ssh myworkspace
```

**Flow:**
```
┌─────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ SSH Client  │────▶│ kubectl         │────▶│ Workspace Pod   │
│ localhost   │     │ port-forward    │     │ :22             │
│ :2222       │     │ :2222 → pod:22  │     │                 │
└─────────────┘     └─────────────────┘     └─────────────────┘
```

### Method 2: Via SSH Proxy (Remote Access)

For accessing workspaces from anywhere without kubectl access.

```bash
ssh myworkspace@proxy.justup.example.com
```

**Flow:**
```
┌─────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ SSH Client  │────▶│ SSH Proxy       │────▶│ Workspace Pod   │
│ any machine │     │ (LoadBalancer)  │     │ :22             │
└─────────────┘     └─────────────────┘     └─────────────────┘
                           │
                           ▼
                    ┌─────────────────┐
                    │ 1. Extract      │
                    │    workspace    │
                    │    from username│
                    │                 │
                    │ 2. Validate     │
                    │    SSH pubkey   │
                    │                 │
                    │ 3. Lookup pod   │
                    │    IP via K8s   │
                    │                 │
                    │ 4. Proxy TCP    │
                    │    to pod:22    │
                    └─────────────────┘
```

### SSH Proxy Authentication

The SSH proxy authenticates users by their SSH public key:

1. User connects: `ssh myworkspace@proxy.justup.example.com`
2. SSH proxy receives the connection
3. Extracts workspace name from SSH username (`myworkspace`)
4. Validates user's public key against registered keys in SQLite
5. If valid, looks up the workspace pod IP
6. Proxies the TCP connection to `<pod-ip>:22`

**Key Registration:**
```bash
# Register your public key
justup ssh-key add ~/.ssh/id_ed25519.pub

# Keys are stored in SQLite at ~/.justup/justup.db (CLI)
# or /var/lib/justup/justup.db (SSH Proxy in K8s)
```

### SSH Config Integration

Add to `~/.ssh/config` for easier access:

```ssh-config
# Direct access via port-forward (requires kubectl)
Host *.justup.local
    ProxyCommand justup ssh %n --stdio
    User dev
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null

# Via SSH proxy (remote access)
Host *.justup.example.com
    User %n
    Port 22
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
```

---

## IDE Integration

### VS Code Remote-SSH

```bash
# Open workspace in VS Code
justup ide vscode myworkspace

# With SSH proxy
justup ide vscode myworkspace --proxy proxy.justup.example.com
```

**What happens:**
1. Verifies workspace is running
2. Constructs VS Code Remote-SSH URL:
   ```
   vscode://vscode-remote/ssh-remote+myworkspace@proxy/home/dev/workspace
   ```
3. Opens the URL, launching VS Code

**Manual setup:**
1. Install "Remote - SSH" extension in VS Code
2. Add SSH config entry (see above)
3. Connect to `myworkspace.justup.example.com`

### JetBrains Gateway

```bash
justup ide jetbrains myworkspace
```

**Output:**
```
JetBrains Gateway connection info for 'myworkspace':

  Host: proxy.justup.example.com
  User: myworkspace
  Port: 22
  Project path: /home/dev/workspace

Open JetBrains Gateway and create a new SSH connection with these details.
```

---

## Configuration

### CLI Configuration

Configuration is stored at `~/.justup/`:

```
~/.justup/
├── config.yaml     # CLI configuration (future)
└── justup.db       # SQLite database (SSH keys, metadata)
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig file |
| `JUSTUP_NAMESPACE` | `justup-workspaces` | Workspace namespace |

---

## Kubernetes Resources

### Namespaces

| Namespace | Purpose |
|-----------|---------|
| `justup-system` | SSH proxy, controller, system components |
| `justup-workspaces` | Workspace pods, PVCs, secrets |

### Resources Created Per Workspace

```yaml
# Pod: ws-<workspace-name>
apiVersion: v1
kind: Pod
metadata:
  name: ws-myworkspace
  namespace: justup-workspaces
  labels:
    justup.io/workspace: myworkspace
spec:
  initContainers:
    - name: git-clone          # Clones the repository
  containers:
    - name: workspace          # Main dev container
    - name: dind               # Optional Docker-in-Docker
  volumes:
    - name: workspace          # PVC mount
    - name: ssh-keys           # SSH authorized_keys
    - name: docker-socket      # DinD socket (if enabled)
```

```yaml
# PVC: ws-<workspace-name>-pvc
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ws-myworkspace-pvc
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 10Gi
```

```yaml
# Secret: ws-<workspace-name>-ssh
apiVersion: v1
kind: Secret
metadata:
  name: ws-myworkspace-ssh
type: Opaque
data:
  authorized_keys: <base64-encoded-keys>
```

### RBAC

The SSH proxy and CLI require cluster-level permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: justup-controller
rules:
  - apiGroups: [""]
    resources: [pods, persistentvolumeclaims, secrets, services]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [""]
    resources: [pods/exec, pods/log, pods/portforward]
    verbs: [get, create]
```

---

## Development

### Project Structure

```
justup/
├── cmd/
│   ├── justup/              # CLI entry point
│   │   └── main.go
│   └── sshproxy/            # SSH proxy entry point
│       └── main.go
├── internal/
│   └── cli/                 # CLI commands (Cobra)
│       ├── root.go          # Root command, version
│       ├── create.go        # justup create
│       ├── list.go          # justup list
│       ├── delete.go        # justup delete
│       ├── ssh.go           # justup ssh
│       ├── start.go         # justup start
│       ├── stop.go          # justup stop
│       ├── sshkey.go        # justup ssh-key
│       └── ide.go           # justup ide
├── pkg/
│   ├── kubernetes/          # Kubernetes client wrapper
│   │   ├── client.go        # K8s client, port-forward
│   │   └── workspace.go     # Workspace CRUD operations
│   ├── database/            # SQLite database
│   │   └── database.go      # SSH keys, workspace metadata
│   └── sshproxy/            # SSH proxy server
│       ├── server.go        # SSH server implementation
│       └── keygen.go        # Host key generation
├── docker/
│   ├── devcontainer/        # Workspace container image
│   │   ├── Dockerfile
│   │   ├── sshd_config
│   │   └── entrypoint.sh
│   └── sshproxy/            # SSH proxy image
│       └── Dockerfile
├── deploy/                  # Kubernetes manifests
│   ├── namespace.yaml
│   ├── rbac.yaml
│   └── sshproxy.yaml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### Make Targets

```bash
make help              # Show all targets

# Building
make build             # Build CLI binary
make build-sshproxy    # Build SSH proxy binary
make build-all-binaries # Build all binaries

# Docker
make docker-build      # Build all Docker images
make docker-push       # Push to registry

# Kubernetes
make k8s-setup         # Create namespaces and RBAC
make k8s-deploy-proxy  # Deploy SSH proxy
make k8s-deploy        # Full deployment
make k8s-clean         # Remove all resources

# Development
make deps              # Download dependencies
make test              # Run tests
make clean             # Remove build artifacts
```

### Adding a New Command

1. Create a new file in `internal/cli/`:

```go
// internal/cli/mycommand.go
package cli

import "github.com/spf13/cobra"

var myCmd = &cobra.Command{
    Use:   "mycommand",
    Short: "Description",
    Run:   runMyCommand,
}

func init() {
    rootCmd.AddCommand(myCmd)
}

func runMyCommand(cmd *cobra.Command, args []string) {
    // Implementation
}
```

2. The command is automatically registered via `init()`.

---

## Troubleshooting

### Pod stuck in Pending

```bash
kubectl describe pod -n justup-workspaces ws-myworkspace
```

Common causes:
- **Insufficient resources**: Check node capacity
- **PVC not bound**: Check storage class availability
- **Image pull error**: Check image name and registry access

### Pod in Init:Error

```bash
kubectl logs -n justup-workspaces ws-myworkspace -c git-clone
```

Common causes:
- **Repository not found**: Check URL is correct
- **Branch not found**: Specify correct branch with `--branch`
- **Private repo**: SSH clone not yet supported for private repos

### Cannot connect via SSH

1. Check pod is running:
   ```bash
   justup list
   ```

2. Check SSH server is listening:
   ```bash
   kubectl exec -n justup-workspaces ws-myworkspace -- ss -tlnp
   ```

3. Check port-forward manually:
   ```bash
   kubectl port-forward -n justup-workspaces ws-myworkspace 2222:22
   ssh -p 2222 dev@localhost
   ```

### Image pull errors

If using custom images, ensure:
1. Image is pushed to an accessible registry
2. Cluster has pull credentials (if private registry)

```bash
# Check events
kubectl get events -n justup-workspaces --sort-by='.lastTimestamp'
```

---

## License

MIT License - See [LICENSE](LICENSE) for details.

---

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request
