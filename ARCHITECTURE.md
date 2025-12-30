# Justup Architecture Documentation

## Overview

Justup is a CLI tool for creating Kubernetes-based development environments. It allows developers to spin up isolated development containers from GitHub repositories and access them via SSH.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              DEVELOPER MACHINE                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────────────────────┐  │
│   │  justup CLI │     │ SSH Client  │     │  VS Code / JetBrains IDE   │  │
│   └──────┬──────┘     └──────┬──────┘     └──────────────┬──────────────┘  │
│          │                   │                           │                  │
└──────────┼───────────────────┼───────────────────────────┼──────────────────┘
           │                   │                           │
           │ kubectl           │ SSH (port 2222)           │ SSH (port 2222)
           │                   │                           │
┌──────────┼───────────────────┼───────────────────────────┼──────────────────┐
│          ▼                   ▼                           ▼                  │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                     KUBERNETES CLUSTER                               │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │                                                                     │   │
│   │   ┌─────────────────────────────────────────────────────────────┐   │   │
│   │   │              NAMESPACE: justup-system                        │   │   │
│   │   ├─────────────────────────────────────────────────────────────┤   │   │
│   │   │                                                             │   │   │
│   │   │   ┌─────────────────────────────────────────────────────┐   │   │   │
│   │   │   │              SSH PROXY DEPLOYMENT                    │   │   │   │
│   │   │   │  ┌─────────────────────────────────────────────┐    │   │   │   │
│   │   │   │  │  - Authenticates SSH connections            │    │   │   │   │
│   │   │   │  │  - Routes to workspace pods by username     │    │   │   │   │
│   │   │   │  │  - Validates keys against SQLite DB         │    │   │   │   │
│   │   │   │  └─────────────────────────────────────────────┘    │   │   │   │
│   │   │   └─────────────────────────────────────────────────────┘   │   │   │
│   │   │                           │                                 │   │   │
│   │   │   ┌───────────────────────┴─────────────────────────────┐   │   │   │
│   │   │   │  LoadBalancer Service (External IP:2222)            │   │   │   │
│   │   │   └─────────────────────────────────────────────────────┘   │   │   │
│   │   │                                                             │   │   │
│   │   └─────────────────────────────────────────────────────────────┘   │   │
│   │                                                                     │   │
│   │   ┌─────────────────────────────────────────────────────────────┐   │   │
│   │   │              NAMESPACE: justup-workspaces                    │   │   │
│   │   ├─────────────────────────────────────────────────────────────┤   │   │
│   │   │                                                             │   │   │
│   │   │   ┌──────────────────┐  ┌──────────────────┐                │   │   │
│   │   │   │  ws-myproject    │  │  ws-another      │   ...          │   │   │
│   │   │   │  ┌────────────┐  │  │  ┌────────────┐  │                │   │   │
│   │   │   │  │ workspace  │  │  │  │ workspace  │  │                │   │   │
│   │   │   │  │ container  │  │  │  │ container  │  │                │   │   │
│   │   │   │  │ (SSH:22)   │  │  │  │ (SSH:22)   │  │                │   │   │
│   │   │   │  └────────────┘  │  │  └────────────┘  │                │   │   │
│   │   │   │  ┌────────────┐  │  │                  │                │   │   │
│   │   │   │  │ dind       │  │  │                  │                │   │   │
│   │   │   │  │ (optional) │  │  │                  │                │   │   │
│   │   │   │  └────────────┘  │  │                  │                │   │   │
│   │   │   └──────────────────┘  └──────────────────┘                │   │   │
│   │   │                                                             │   │   │
│   │   └─────────────────────────────────────────────────────────────┘   │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Component Details

### 1. CLI (`cmd/justup`, `internal/cli`)

The CLI is the primary interface for users. Built with Cobra framework.

#### Commands

| Command | Description | What it does |
|---------|-------------|--------------|
| `justup create <url>` | Create workspace | Creates Pod + PVC + Secret in Kubernetes |
| `justup list` | List workspaces | Queries pods with `justup.io/workspace` label |
| `justup delete <name>` | Delete workspace | Removes Pod, PVC, and Secret |
| `justup ssh <name>` | Connect via SSH | Port-forwards and runs SSH |
| `justup start <name>` | Start stopped workspace | Recreates pod from PVC metadata |
| `justup stop <name>` | Stop workspace | Deletes pod, keeps PVC |
| `justup ssh-key add` | Add SSH key | Stores public key in SQLite |
| `justup ssh-key list` | List SSH keys | Shows registered keys |

#### Database Location

```
~/.justup/justup.db    # SQLite database
```

Contains:
- `users` table: User information
- `ssh_keys` table: Public keys with fingerprints

---

### 2. Kubernetes Client (`pkg/kubernetes`)

Handles all Kubernetes operations using `client-go`.

#### Workspace Creation Flow

```
justup create github.com/user/repo --name myproject
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. VALIDATE INPUT                                           │
│    - Check workspace name is valid (lowercase, alphanumeric)│
│    - Normalize GitHub URL (add https://, .git suffix)       │
└─────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. LOAD SSH KEYS FROM LOCAL DATABASE                        │
│    - Open ~/.justup/justup.db                               │
│    - Get default user                                       │
│    - Load all SSH public keys for user                      │
│    - Join keys with newlines                                │
└─────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. CREATE KUBERNETES RESOURCES                              │
│                                                             │
│    a) PersistentVolumeClaim (ws-myproject-pvc)              │
│       - Storage for /home/dev/workspace                     │
│       - Labels: justup.io/workspace=myproject               │
│       - Annotations: git URL, branch                        │
│                                                             │
│    b) Secret (ws-myproject-ssh)                             │
│       - Contains authorized_keys content                    │
│       - Mounted read-only in pod                            │
│                                                             │
│    c) Pod (ws-myproject)                                    │
│       - Init container: clones git repo                     │
│       - Main container: devcontainer with SSH               │
│       - Optional: DinD sidecar container                    │
│       - Volumes: PVC, Secret, emptyDirs                     │
└─────────────────────────────────────────────────────────────┘
```

#### Pod Specification

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ws-myproject
  namespace: justup-workspaces
  labels:
    app.kubernetes.io/name: justup-workspace
    app.kubernetes.io/instance: myproject
    justup.io/workspace: myproject
  annotations:
    justup.io/git-url: https://github.com/user/repo.git
    justup.io/branch: main
    # AppArmor disabled for SSH compatibility
    container.apparmor.security.beta.kubernetes.io/workspace: unconfined
spec:
  initContainers:
    - name: git-clone
      image: alpine/git:latest
      command: ["/bin/sh", "-c"]
      args:
        - |
          if [ ! -d /workspace/.git ]; then
            git clone --branch main https://github.com/user/repo.git /workspace
          fi
      volumeMounts:
        - name: workspace
          mountPath: /workspace

  containers:
    - name: workspace
      image: ghcr.io/rahulvramesh/justup/devcontainer:latest
      imagePullPolicy: Always
      ports:
        - name: ssh
          containerPort: 22
      env:
        - name: JUSTUP_WORKSPACE
          value: myproject
        - name: GIT_URL
          value: https://github.com/user/repo.git
        - name: GIT_BRANCH
          value: main
        - name: DOCKER_HOST          # Only if DinD enabled
          value: tcp://localhost:2375
      volumeMounts:
        - name: workspace
          mountPath: /home/dev/workspace
        - name: ssh-keys
          mountPath: /etc/justup/ssh-keys
          readOnly: true
      resources:
        requests:
          cpu: "1"
          memory: 2Gi
        limits:
          cpu: "1"
          memory: 2Gi

    - name: dind                      # Only if --dind flag
      image: docker:24-dind
      securityContext:
        privileged: true
      volumeMounts:
        - name: docker-socket
          mountPath: /var/run
        - name: docker-storage
          mountPath: /var/lib/docker

  volumes:
    - name: workspace
      persistentVolumeClaim:
        claimName: ws-myproject-pvc
    - name: ssh-keys
      secret:
        secretName: ws-myproject-ssh
        defaultMode: 0600
    - name: docker-socket             # Only if DinD
      emptyDir: {}
    - name: docker-storage            # Only if DinD
      emptyDir: {}
```

---

### 3. Dev Container (`docker/devcontainer`)

The container image that users develop in.

#### Dockerfile Summary

```dockerfile
FROM debian:bookworm-slim

# Install: openssh-server, git, vim, zsh, docker-cli, build-essential, etc.

# Create 'dev' user with:
#   - UID/GID 1000
#   - Home: /home/dev
#   - Shell: /bin/zsh
#   - Passwordless sudo

# Configure SSH server
COPY sshd_config /etc/ssh/sshd_config

# Install oh-my-zsh for better shell experience

COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/sbin/sshd", "-D", "-e"]
```

#### Entrypoint Logic (`entrypoint.sh`)

```bash
#!/bin/bash
set -e

# 1. Create /run/sshd directory (required by sshd)
mkdir -p /run/sshd
chmod 755 /run/sshd

# 2. Unlock dev user account
#    SSH rejects locked accounts even with valid keys
#    The account is locked by default (no password set)
passwd -u dev 2>/dev/null || usermod -p '*' dev 2>/dev/null || true

# 3. Generate SSH host keys if they don't exist
if [ ! -f /etc/ssh/ssh_host_rsa_key ]; then
    ssh-keygen -t rsa -b 4096 -f /etc/ssh/ssh_host_rsa_key -N ''
fi
if [ ! -f /etc/ssh/ssh_host_ed25519_key ]; then
    ssh-keygen -t ed25519 -f /etc/ssh/ssh_host_ed25519_key -N ''
fi

# 4. Copy authorized_keys from read-only secret mount
#    Secrets are mounted read-only, so we copy to writable location
SSH_KEYS_SOURCE="/etc/justup/ssh-keys"
SSH_DIR="/home/dev/.ssh"

mkdir -p "$SSH_DIR"
chown dev:dev "$SSH_DIR"
chmod 700 "$SSH_DIR"

if [ -f "$SSH_KEYS_SOURCE/authorized_keys" ]; then
    cp "$SSH_KEYS_SOURCE/authorized_keys" "$SSH_DIR/authorized_keys"
    chown dev:dev "$SSH_DIR/authorized_keys"
    chmod 600 "$SSH_DIR/authorized_keys"
fi

# 5. Fix workspace ownership
chown -R dev:dev /home/dev/workspace 2>/dev/null || true

# 6. Clone repo if not already cloned
if [ -n "$GIT_URL" ] && [ ! -d /home/dev/workspace/.git ]; then
    su - dev -c "git clone --branch ${GIT_BRANCH:-main} $GIT_URL /home/dev/workspace"
fi

# 7. Start SSH server
exec "$@"
```

#### SSH Configuration (`sshd_config`)

```
Port 22
ListenAddress 0.0.0.0
Protocol 2

# Host keys
HostKey /etc/ssh/ssh_host_rsa_key
HostKey /etc/ssh/ssh_host_ed25519_key

# Authentication
PermitRootLogin no
PubkeyAuthentication yes
PasswordAuthentication no
ChallengeResponseAuthentication no
UsePAM no                              # Disabled for container compatibility

# Authorized keys location
AuthorizedKeysFile /home/dev/.ssh/authorized_keys

# Forwarding (for IDE port forwarding)
X11Forwarding yes
AllowAgentForwarding yes
AllowTcpForwarding yes
PermitTunnel yes

# Performance
UseDNS no

# Keep connections alive
ClientAliveInterval 60
ClientAliveCountMax 3

# Allow only dev user
AllowUsers dev
```

---

### 4. SSH Proxy (`pkg/sshproxy`)

The SSH proxy is the most complex component. It enables direct SSH access to workspaces without port-forwarding.

#### Why Do We Need a Proxy?

Without proxy:
```
Developer → kubectl port-forward → Pod:22
            (must keep running)
```

With proxy:
```
Developer → SSH Proxy (external IP) → Pod:22
            (always available)
```

Benefits:
- Single stable endpoint for all workspaces
- Works with VS Code Remote SSH, JetBrains Gateway
- No need to run kubectl port-forward

#### Proxy Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              SSH PROXY SERVER                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   INCOMING CONNECTION                                                       │
│   ssh -p 2222 myproject@142.132.221.35                                     │
│              └────┬────┘                                                    │
│                   │                                                         │
│                   ▼                                                         │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │ 1. SSH HANDSHAKE                                                    │   │
│   │    - Server presents host key                                       │   │
│   │    - Client verifies (or accepts)                                   │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                   │                                                         │
│                   ▼                                                         │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │ 2. EXTRACT WORKSPACE NAME FROM USERNAME                             │   │
│   │    Username: "myproject" → Workspace: "myproject"                   │   │
│   │    Pod name will be: "ws-myproject"                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                   │                                                         │
│                   ▼                                                         │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │ 3. PUBLIC KEY AUTHENTICATION                                        │   │
│   │                                                                     │   │
│   │    Client offers public key                                         │   │
│   │              │                                                      │   │
│   │              ▼                                                      │   │
│   │    ┌─────────────────────────────────────────────────┐              │   │
│   │    │ Calculate SHA256 fingerprint of offered key    │              │   │
│   │    │ Example: SHA256:gBjTiMf9lW9obcvtO/ZAS6m0LNI... │              │   │
│   │    └─────────────────────────────────────────────────┘              │   │
│   │              │                                                      │   │
│   │              ▼                                                      │   │
│   │    ┌─────────────────────────────────────────────────┐              │   │
│   │    │ Query SQLite database:                         │              │   │
│   │    │ SELECT * FROM ssh_keys                         │              │   │
│   │    │ WHERE fingerprint = 'SHA256:gBjT...'           │              │   │
│   │    └─────────────────────────────────────────────────┘              │   │
│   │              │                                                      │   │
│   │              ▼                                                      │   │
│   │    ┌─────────────────────────────────────────────────┐              │   │
│   │    │ If found: Authentication SUCCESS               │              │   │
│   │    │ If not found: Authentication FAILED            │              │   │
│   │    └─────────────────────────────────────────────────┘              │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                   │                                                         │
│                   ▼ (if auth successful)                                    │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │ 4. LOOKUP WORKSPACE POD                                             │   │
│   │                                                                     │   │
│   │    Query Kubernetes API:                                            │   │
│   │    GET /api/v1/namespaces/justup-workspaces/pods/ws-myproject      │   │
│   │                                                                     │   │
│   │    Extract Pod IP: 10.42.0.150                                      │   │
│   │                                                                     │   │
│   │    Verify status: Running                                           │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                   │                                                         │
│                   ▼                                                         │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │ 5. CONNECT TO WORKSPACE POD                                         │   │
│   │                                                                     │   │
│   │    Open TCP connection to Pod IP:22                                 │   │
│   │    10.42.0.150:22                                                   │   │
│   │                                                                     │   │
│   │    Establish SSH connection as "dev" user                           │   │
│   │    Using proxy's host key for authentication                        │   │
│   │                                                                     │   │
│   │    NOTE: Proxy's public key must be in pod's authorized_keys!       │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                   │                                                         │
│                   ▼                                                         │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │ 6. PROXY SSH TRAFFIC                                                │   │
│   │                                                                     │   │
│   │    ┌─────────┐         ┌─────────┐         ┌─────────┐              │   │
│   │    │ Client  │ ◄─────► │  Proxy  │ ◄─────► │   Pod   │              │   │
│   │    └─────────┘         └─────────┘         └─────────┘              │   │
│   │                                                                     │   │
│   │    Bidirectional proxying of:                                       │   │
│   │    - SSH channels (shell, exec, subsystem)                          │   │
│   │    - Channel requests (pty-req, env, shell)                         │   │
│   │    - Global requests (keepalive)                                    │   │
│   │    - Data streams (stdin, stdout, stderr)                           │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Proxy Code Flow (`pkg/sshproxy/server.go`)

```go
// 1. Server Initialization
func NewServer(config *Config) (*Server, error) {
    // Load host key from /etc/justup/ssh_host_ed25519_key
    hostKey, err := loadOrGenerateHostKey(config.HostKeyPath)

    // Open SQLite database for key validation
    db, err := database.Open(config.DatabasePath)

    // Create Kubernetes client for pod lookups
    k8sClient, err := kubernetes.NewClient()

    // Configure SSH server
    sshConfig := &ssh.ServerConfig{
        PublicKeyCallback: server.publicKeyCallback,  // Key validation
        ServerVersion:     "SSH-2.0-JustupProxy",
    }
    sshConfig.AddHostKey(hostKey)

    return &Server{...}, nil
}

// 2. Key Validation Callback
func (s *Server) publicKeyCallback(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
    fingerprint := ssh.FingerprintSHA256(key)
    workspaceName := conn.User()  // Username = workspace name

    // Query database for key
    sshKey, err := s.db.GetSSHKeyByFingerprint(fingerprint)
    if err != nil {
        return nil, fmt.Errorf("unknown public key")
    }

    // Update last used timestamp
    s.db.UpdateSSHKeyLastUsed(sshKey.ID)

    return &ssh.Permissions{
        Extensions: map[string]string{
            "user-id":     sshKey.UserID,
            "fingerprint": fingerprint,
        },
    }, nil
}

// 3. Connection Handler
func (s *Server) handleConnection(ctx context.Context, netConn net.Conn) {
    // SSH handshake
    sshConn, chans, reqs, err := ssh.NewServerConn(netConn, s.sshConfig)

    // Get workspace name from username
    workspaceName := sshConn.User()  // e.g., "myproject"

    // Lookup workspace pod in Kubernetes
    ws, err := s.k8sClient.GetWorkspace(ctx, workspaceName)
    // ws.PodIP = "10.42.0.150"

    // Connect to workspace pod
    targetAddr := fmt.Sprintf("%s:22", ws.PodIP)  // "10.42.0.150:22"
    targetConn, err := net.Dial("tcp", targetAddr)

    // Establish SSH client connection to pod
    targetSSHConn, targetChans, targetReqs, err := ssh.NewClientConn(
        targetConn,
        targetAddr,
        &ssh.ClientConfig{
            User: "dev",
            Auth: []ssh.AuthMethod{
                ssh.PublicKeys(s.hostSigner),  // Use proxy's key
            },
            HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        },
    )

    // Proxy all traffic bidirectionally
    go proxyRequests(reqs, targetSSHConn)
    go proxyRequests(targetReqs, sshConn)
    go proxyChannels(chans, targetSSHConn)
    go proxyChannels(targetChans, sshConn)
}
```

#### Proxy Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: justup-sshproxy
  namespace: justup-system
spec:
  replicas: 1
  template:
    spec:
      serviceAccountName: justup-controller  # Needs pod list/get permissions
      containers:
        - name: sshproxy
          image: ghcr.io/rahulvramesh/justup/sshproxy:latest
          imagePullPolicy: Always
          ports:
            - containerPort: 2222
          args:
            - --addr=:2222
            - --host-key=/etc/justup/ssh_host_ed25519_key
            - --db=/var/lib/justup/justup.db
          volumeMounts:
            - name: data
              mountPath: /var/lib/justup      # SQLite database
            - name: host-key
              mountPath: /etc/justup          # SSH host key
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: justup-sshproxy-data
        - name: host-key
          secret:
            secretName: justup-ssh-host-key
            defaultMode: 0444                 # Readable by non-root
---
apiVersion: v1
kind: Service
metadata:
  name: justup-sshproxy
spec:
  type: LoadBalancer
  ports:
    - port: 2222          # External port
      targetPort: 2222    # Container port
```

#### Important: Proxy Key in Workspaces

The proxy authenticates to workspace pods using its host key. This key must be in each workspace's `authorized_keys`:

```
# In each workspace pod's /home/dev/.ssh/authorized_keys:
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEZr... user-key
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIxyz... justup-proxy  <-- REQUIRED
```

**Current limitation:** The proxy's public key must be manually added to workspaces or included in the workspace creation process.

---

### 5. Database (`pkg/database`)

SQLite database for storing user information and SSH keys.

#### Schema

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ssh_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    public_key TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

#### Key Operations

```go
// Add SSH key
func (db *DB) AddSSHKey(userID, name, publicKey string) (*SSHKey, error) {
    // Parse public key to validate format
    key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))

    // Calculate fingerprint
    fingerprint := ssh.FingerprintSHA256(key)

    // Insert into database
    id := uuid.New().String()
    _, err = db.conn.Exec(`
        INSERT INTO ssh_keys (id, user_id, name, public_key, fingerprint)
        VALUES (?, ?, ?, ?, ?)
    `, id, userID, name, publicKey, fingerprint)

    return &SSHKey{...}, nil
}

// Lookup key by fingerprint (used by proxy)
func (db *DB) GetSSHKeyByFingerprint(fingerprint string) (*SSHKey, error) {
    row := db.conn.QueryRow(`
        SELECT id, user_id, name, public_key, fingerprint, created_at, last_used_at
        FROM ssh_keys WHERE fingerprint = ?
    `, fingerprint)
    // ...
}
```

---

### 6. Docker-in-Docker (DinD)

When `--dind` flag is used, a Docker daemon runs as a sidecar container.

#### Architecture

```
┌─────────────────────────────────────────────┐
│              POD: ws-myproject              │
├─────────────────────────────────────────────┤
│                                             │
│  ┌─────────────────────────────────────┐    │
│  │     CONTAINER: workspace            │    │
│  │                                     │    │
│  │  DOCKER_HOST=tcp://localhost:2375   │────┼───┐
│  │                                     │    │   │
│  │  $ docker ps                        │    │   │
│  │  $ docker build .                   │    │   │
│  │  $ docker run nginx                 │    │   │
│  └─────────────────────────────────────┘    │   │
│                                             │   │
│  ┌─────────────────────────────────────┐    │   │
│  │     CONTAINER: dind                 │    │   │
│  │     (docker:24-dind)                │    │   │
│  │                                     │    │   │
│  │  privileged: true                   │◄───┼───┘
│  │                                     │    │   TCP:2375
│  │  Docker daemon listening on:        │    │
│  │  - tcp://0.0.0.0:2375               │    │
│  │  - unix:///var/run/docker.sock      │    │
│  │                                     │    │
│  │  /var/lib/docker (emptyDir)         │    │
│  └─────────────────────────────────────┘    │
│                                             │
└─────────────────────────────────────────────┘
```

#### Why TCP Instead of Socket?

Originally, we tried sharing `/var/run/docker.sock` via emptyDir volume:
- DinD mounts emptyDir at `/var/run`
- Workspace mounts same emptyDir at `/var/run/docker.sock`

**Problem:** Mounting emptyDir at a file path creates a directory, not a file. The socket file created by DinD isn't visible to the workspace.

**Solution:** Use TCP connection instead:
```bash
export DOCKER_HOST=tcp://localhost:2375
```

This works because both containers share the same network namespace (localhost).

---

## Setup Procedures

### Initial Cluster Setup

```bash
# 1. Create namespaces
kubectl apply -f deploy/namespace.yaml

# 2. Create RBAC (ServiceAccount + ClusterRole + Binding)
kubectl apply -f deploy/rbac.yaml

# 3. Generate SSH host key for proxy
ssh-keygen -t ed25519 -f /tmp/ssh_host_ed25519_key -N ''

# 4. Create secret with host key
kubectl create secret generic justup-ssh-host-key \
  --namespace justup-system \
  --from-file=ssh_host_ed25519_key=/tmp/ssh_host_ed25519_key

# 5. Deploy SSH proxy
kubectl apply -f deploy/sshproxy.yaml

# 6. Wait for proxy to be ready
kubectl rollout status deployment/justup-sshproxy -n justup-system
```

### CLI Setup

```bash
# 1. Build CLI
make build

# 2. Add your SSH key
./bin/justup ssh-key add ~/.ssh/id_ed25519.pub

# 3. Sync database to proxy
POD=$(kubectl get pods -n justup-system -l app=justup-sshproxy -o jsonpath='{.items[0].metadata.name}')
kubectl cp ~/.justup/justup.db justup-system/$POD:/var/lib/justup/justup.db
kubectl rollout restart deployment justup-sshproxy -n justup-system
```

### Create and Connect to Workspace

```bash
# Create workspace
./bin/justup create github.com/user/repo --name myproject

# Option 1: Connect via CLI (uses port-forward)
./bin/justup ssh myproject

# Option 2: Connect via proxy (direct)
ssh -p 2222 myproject@<PROXY_EXTERNAL_IP>

# Option 3: VS Code Remote SSH
# Add to ~/.ssh/config:
#   Host *.justup
#       HostName <PROXY_EXTERNAL_IP>
#       Port 2222
#       User %n
# Then connect to: myproject.justup
```

---

## Troubleshooting

### SSH: "Permission denied (publickey)"

**Cause 1:** User account is locked
```bash
kubectl exec -n justup-workspaces ws-<name> -c workspace -- usermod -p '*' dev
```

**Cause 2:** SSH key not in authorized_keys
```bash
kubectl exec -n justup-workspaces ws-<name> -c workspace -- cat /home/dev/.ssh/authorized_keys
```

**Cause 3:** Proxy key not in workspace (for proxy connections)
```bash
# Get proxy public key
ssh-keygen -y -f /tmp/ssh_host_ed25519_key

# Add to workspace
kubectl exec -n justup-workspaces ws-<name> -c workspace -- \
  sh -c 'echo "ssh-ed25519 AAAA... justup-proxy" >> /home/dev/.ssh/authorized_keys'
```

### SSH Proxy: "Key not found"

The proxy's database doesn't have your SSH key.

```bash
# Sync local database to proxy
kubectl cp ~/.justup/justup.db justup-system/<proxy-pod>:/var/lib/justup/justup.db
kubectl rollout restart deployment justup-sshproxy -n justup-system
```

### Docker: "Cannot connect to the Docker daemon"

For DinD workspaces, set the Docker host:
```bash
export DOCKER_HOST=tcp://localhost:2375
docker ps
```

### Pod: "CrashLoopBackOff"

Check logs:
```bash
kubectl logs -n justup-workspaces ws-<name> -c workspace
kubectl logs -n justup-workspaces ws-<name> -c dind  # If DinD enabled
```

---

## Security Considerations

1. **SSH Keys:** Stored in SQLite, synced to proxy. Consider encrypting at rest.

2. **Proxy Host Key:** Generated once, stored in Kubernetes secret. Clients may get host key warnings if regenerated.

3. **DinD Privileged Mode:** Required for Docker, but grants root capabilities. Use with caution.

4. **AppArmor Disabled:** Required for SSH to work in some environments. Consider alternatives.

5. **Network Policy:** Consider adding NetworkPolicy to restrict workspace-to-workspace communication.

6. **RBAC:** The `justup-controller` ServiceAccount has cluster-wide pod read access. Scope down if possible.
