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

# Fix permissions for SSH directory
if [ -d /home/dev/.ssh ]; then
    chown -R dev:dev /home/dev/.ssh
    chmod 700 /home/dev/.ssh
    if [ -f /home/dev/.ssh/authorized_keys ]; then
        chmod 600 /home/dev/.ssh/authorized_keys
    fi
fi

# Fix ownership of workspace directory
if [ -d /home/dev/workspace ]; then
    chown -R dev:dev /home/dev/workspace
fi

# Clone repository if GIT_URL is set and workspace is empty
if [ -n "$GIT_URL" ] && [ ! -d /home/dev/workspace/.git ]; then
    echo "Cloning repository: $GIT_URL (branch: ${GIT_BRANCH:-main})"
    su - dev -c "git clone --branch ${GIT_BRANCH:-main} $GIT_URL /home/dev/workspace" || true
fi

echo "Starting SSH server..."
exec "$@"
