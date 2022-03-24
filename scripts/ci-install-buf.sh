#!/bin/bash

echo "Installing dependencies for goos:$1"

# Make sure you grab the latest version
VERSION=1.1.1
DOWNLOAD=https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf

# Install buf based on goos
if [ $1 = "windows" ]; then
  DOWNLOAD="${DOWNLOAD}-Windows-x86_64.exe"
  wget --quiet "${DOWNLOAD}" -O /tmp/buf.exe
  chmod +x /tmp/buf.exe
  ls /tmp
elif [ $1 = "darwin" ]; then
  DOWNLOAD="${DOWNLOAD}-Darwin-x86_64.tar.gz"
  wget --quiet "${DOWNLOAD}" -O - | tar -xz -C /tmp
  chmod +x /tmp/buf/bin/buf
  ls /tmp
else
  DOWNLOAD="${DOWNLOAD}-Linux-x86_64.tar.gz"
  wget --quiet "${DOWNLOAD}" -O - | tar -xz -C /tmp
  chmod +x /tmp/buf/bin/buf
  ls /tmp
fi

# Simple smoke test to ensure buf is installed
# buf --version