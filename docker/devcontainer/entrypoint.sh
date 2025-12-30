#!/bin/bash
set -e

# Generate SSH host keys if they don't exist
if [ ! -f /etc/ssh/ssh_host_rsa_key ]; then
    echo "Generating RSA host key..."
    ssh-keygen -t rsa -b 4096 -f /etc/ssh/ssh_host_rsa_key -N ''
fi

if [ ! -f /etc/ssh/ssh_host_ed25519_key ]; then
    echo "Generating ED25519 host key..."
    ssh-keygen -t ed25519 -f /etc/ssh/ssh_host_ed25519_key -N ''
fi

# Setup SSH authorized_keys from mounted secret
# The secret is mounted read-only, so we copy to a writable location
SSH_KEYS_SOURCE="/etc/justup/ssh-keys"
SSH_DIR="/home/dev/.ssh"

mkdir -p "$SSH_DIR"
chown dev:dev "$SSH_DIR"
chmod 700 "$SSH_DIR"

# Copy authorized_keys from mounted secret if it exists
if [ -f "$SSH_KEYS_SOURCE/authorized_keys" ]; then
    echo "Setting up SSH authorized keys..."
    cp "$SSH_KEYS_SOURCE/authorized_keys" "$SSH_DIR/authorized_keys"
    chown dev:dev "$SSH_DIR/authorized_keys"
    chmod 600 "$SSH_DIR/authorized_keys"
fi

# Fix ownership of workspace directory (ignore errors for mounted volumes)
if [ -d /home/dev/workspace ]; then
    chown -R dev:dev /home/dev/workspace 2>/dev/null || true
fi

# Clone repository if GIT_URL is set and workspace is empty
if [ -n "$GIT_URL" ] && [ ! -d /home/dev/workspace/.git ]; then
    echo "Cloning repository: $GIT_URL (branch: ${GIT_BRANCH:-main})"
    su - dev -c "git clone --branch ${GIT_BRANCH:-main} $GIT_URL /home/dev/workspace" || true
fi

echo "Starting SSH server..."
exec "$@"
