# Justup Dev Container
# Debian-based development environment with SSH access

FROM debian:bookworm-slim

ARG USERNAME=dev
ARG USER_UID=1000
ARG USER_GID=1000

# Avoid prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install essential packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    # SSH server
    openssh-server \
    # Essential tools
    curl \
    wget \
    git \
    vim \
    nano \
    htop \
    tmux \
    less \
    # Shell
    zsh \
    bash-completion \
    # Build tools
    build-essential \
    cmake \
    pkg-config \
    # Networking
    net-tools \
    dnsutils \
    iputils-ping \
    ca-certificates \
    # Docker CLI (communicates with DinD sidecar)
    docker.io \
    # Process tools
    procps \
    # Sudo
    sudo \
    # Locales
    locales \
    && rm -rf /var/lib/apt/lists/*

# Configure locale
RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen && locale-gen
ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8

# Create non-root user
RUN groupadd --gid ${USER_GID} ${USERNAME} \
    && useradd --uid ${USER_UID} --gid ${USER_GID} -m -s /bin/zsh ${USERNAME} \
    && echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/${USERNAME} \
    && chmod 0440 /etc/sudoers.d/${USERNAME}

# Configure SSH
RUN mkdir -p /var/run/sshd \
    && mkdir -p /home/${USERNAME}/.ssh \
    && chmod 700 /home/${USERNAME}/.ssh \
    && chown -R ${USERNAME}:${USERNAME} /home/${USERNAME}/.ssh

# SSH configuration
COPY sshd_config /etc/ssh/sshd_config

# Create workspace directory
RUN mkdir -p /home/${USERNAME}/workspace \
    && chown -R ${USERNAME}:${USERNAME} /home/${USERNAME}/workspace

# Install oh-my-zsh for better shell experience
USER ${USERNAME}
WORKDIR /home/${USERNAME}
RUN sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended || true

# Set up basic .zshrc
RUN echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc \
    && echo 'export DOCKER_HOST="unix:///var/run/docker.sock"' >> ~/.zshrc \
    && echo 'cd ~/workspace 2>/dev/null || true' >> ~/.zshrc

# Switch back to root
USER root

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose SSH port
EXPOSE 22

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD pgrep -x sshd > /dev/null || exit 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/sbin/sshd", "-D", "-e"]
