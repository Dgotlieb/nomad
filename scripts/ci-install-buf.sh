#!/bin/bash

echo "Installing dependencies for goos:$1"

# Make sure you grab the latest version
VERSION=1.1.1
DOWNLOAD=https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf

# In GHA, this evaluates to /home/runner/work/nomad/nomad/tmp
mkdir -p $PWD/tmp

echo "what is home: $HOME"

# Install buf based on goos
if [ $1 = "windows" ]; then
  DOWNLOAD="${DOWNLOAD}-Windows-x86_64.exe"
  wget --quiet "${DOWNLOAD}" -O "$HOME/.local/bin/buf.exe"
  chmod +x $HOME/.local/bin/buf.exe
elif [ $1 = "darwin" ]; then
  DOWNLOAD="${DOWNLOAD}-Darwin-x86_64.tar.gz"
  wget --quiet "${DOWNLOAD}" -O - | tar -xz -C /tmp
  mv /tmp/buf/bin/buf "$HOME/.local/bin"
  chmod +x $HOME/.local/bin/buf
else
  DOWNLOAD="${DOWNLOAD}-Linux-x86_64.tar.gz"
  wget --quiet "${DOWNLOAD}" -O - | tar -xz -C /tmp
  mv /tmp/buf/bin/buf "$HOME/.local/bin"
  chmod +x $HOME/.local/bin/buf
fi
