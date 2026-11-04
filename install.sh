#!/bin/bash
# Rovunet Panel installer — download the rovunet binary and run `rovunet install`
set -e

# Releases carrying the rovunet binary. Override with ROVUNET_REPO=owner/repo.
REPO="${ROVUNET_REPO:-msVPNco/rovunet-panel}"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[1;36m'
NC='\033[0m'

info()    { echo -e "${GREEN}[+]${NC} $1"; }
error()   { echo -e "${RED}[-]${NC} $1"; exit 1; }

# Check root
[[ $EUID -ne 0 ]] && error "This script must be run as root (sudo)"

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *)       error "Unsupported architecture: $ARCH" ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
[[ "$OS" != "linux" ]] && error "Rovunet Panel only supports Linux"

BINARY="rovunet-${OS}-${ARCH}"

# Override with: ROVUNET_RELEASE_TAG=v1.8.0 bash install.sh
RELEASE_TAG="${ROVUNET_RELEASE_TAG:-}"
CHANNEL=""  # ← set to "dev" on dev branch, empty on main

if [[ -n "$RELEASE_TAG" ]]; then
    URL="https://github.com/${REPO}/releases/download/${RELEASE_TAG}/${BINARY}"
elif [[ "$CHANNEL" == "dev" ]]; then
    # Find the latest dev pre-release tag via GitHub API
    DEV_TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" \
        | grep -o '"tag_name": *"[^"]*-dev"' | head -1 | grep -o '"[^"]*-dev"' | tr -d '"')
    if [[ -n "$DEV_TAG" ]]; then
        URL="https://github.com/${REPO}/releases/download/${DEV_TAG}/${BINARY}"
        info "Dev channel: using release ${DEV_TAG}"
    else
        URL="https://github.com/${REPO}/releases/latest/download/${BINARY}"
        info "No dev release found, falling back to latest stable"
    fi
else
    URL="https://github.com/${REPO}/releases/latest/download/${BINARY}"
fi

echo -e "${CYAN}"
cat <<'BANNER'
 ____                                     _
|  _ \   ___  __   __ _   _  _ __    ___ | |_
| |_) | / _ \ \ \ / /| | | || '_ \  / _ \| __|
|  _ < | (_) | \ V / | |_| || | | ||  __/| |_
|_| \_\ \___/   \_/   \__,_||_| |_| \___| \__|
                Rovunet Panel
BANNER
echo -e "${NC}"

# Kill any running panel/tunnel processes and remove the old binary
killall rovunet 2>/dev/null || true
killall slipgate 2>/dev/null || true
killall dnstt-server 2>/dev/null || true
killall slipstream-server 2>/dev/null || true
rm -f "${INSTALL_DIR}/rovunet"

info "Downloading rovunet ($OS/$ARCH)..."
if command -v curl &>/dev/null; then
    curl -fsSL "$URL" -o "${INSTALL_DIR}/rovunet"
elif command -v wget &>/dev/null; then
    wget -qO "${INSTALL_DIR}/rovunet" "$URL"
else
    error "Neither curl nor wget found"
fi

chmod +x "${INSTALL_DIR}/rovunet"

info "Running rovunet install..."
info "If installation fails, run 'sudo rovunet install' to retry."
info "When it finishes, run 'sudo rovunet' to see the menu."

# exec so rovunet replaces this shell and becomes sudo's direct child.
# Under `curl | sudo bash`, sudo (use_pty, and always with sudo-rs on
# Ubuntu 25.10+) starts the command in the pty's background process group.
# The first prompt's tcsetattr then stops rovunet with SIGTTOU; sudo only
# resumes it in the foreground (with the user's terminal in raw mode) if it
# can see the stop, which it can't when rovunet is a grandchild behind a
# non-interactive bash.
exec "${INSTALL_DIR}/rovunet" install </dev/tty
