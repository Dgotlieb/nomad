#!/usr/bin/env bash

set -o errexit

# Make sure you grab the latest version
VERSION=1.1.1
DOWNLOAD=https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf-Linux-x86_64

function install() {
  if command -v buf >/dev/null; then
    if [[ "${VERSION}" = "$(buf  --version)" ]] ; then
      return
    fi
  fi

  # Install to a different locatin when called from GHA runners
  if [[ $1 == "GHA_RUNNERS" ]]; then
    # Download
    curl -sSL --fail "$DOWNLOAD" -o $RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/bin/buf
    # Make executable
    chmod +x $RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/bin/buf
  else
    # Download
    curl -sSL --fail "$DOWNLOAD" -o /tmp/buf
    # Make executable
    chmod +x /tmp/buf
    # Move buf to /usr/bin
    mv /tmp/buf /usr/bin/buf
  fi
}

install
