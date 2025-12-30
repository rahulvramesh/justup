# Justup Test Commands

## Basic Commands

```bash
# Check version and help
./bin/justup version
./bin/justup --help
```

## SSH Key Management

```bash
# List SSH keys (should show your registered key)
./bin/justup ssh-key list

# Add SSH key (if not already added)
./bin/justup ssh-key add ~/.ssh/id_ed25519.pub
```

## Workspace Management

```bash
# List workspaces
./bin/justup list
./bin/justup list -a  # include stopped workspaces

# Create a new workspace
./bin/justup create github.com/gin-gonic/gin --name gin-test --branch master

# Check workspace status
./bin/justup list

# SSH into workspace (interactive)
./bin/justup ssh gin-test

# Inside SSH session, verify:
#    whoami          -> should show "dev"
#    pwd             -> should show "/home/dev"
#    ls workspace/   -> should show cloned repo
#    exit            -> to exit

# Stop workspace (keeps data)
./bin/justup stop gin-test

# Start workspace again
./bin/justup start gin-test

# Delete workspace
./bin/justup delete gin-test -f
```

## Docker-in-Docker Testing

```bash
# Create workspace with Docker-in-Docker
./bin/justup create github.com/gin-gonic/gin --name gin-dind --branch master --dind

# Test Docker inside workspace
./bin/justup ssh gin-dind
# Inside: docker ps    -> should work if DinD is running

# Cleanup
./bin/justup delete gin-dind -f
```

## SSH Proxy (Direct IDE Access)

The SSH proxy allows direct access via `ssh workspace@proxy-ip`:

```bash
# Deploy SSH proxy
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/sshproxy.yaml

# Get proxy external IP
kubectl get svc -n justup-system justup-sshproxy

# Copy local database to proxy (for key sync)
kubectl cp ~/.justup/justup.db justup-system/$(kubectl get pods -n justup-system -l app=justup-sshproxy -o jsonpath='{.items[0].metadata.name}'):/var/lib/justup/justup.db

# Restart proxy to load database
kubectl rollout restart deployment justup-sshproxy -n justup-system

# SSH via proxy (use workspace name as username)
ssh -p 2222 gin-test@<PROXY_EXTERNAL_IP>
```

### VS Code SSH Config

Add to `~/.ssh/config`:
```
Host *.justup
    HostName 142.132.221.35  # Your proxy external IP
    Port 2222
    User %n
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
```

Then in VS Code: `Remote-SSH: Connect to Host` → `gin-test.justup`

## Quick Test Sequence

```bash
./bin/justup list && \
./bin/justup ssh gin-test
```
