#!/usr/bin/env bash

echo "Installing dependencies for goos:$1"

# Make sure you grab the latest version
VERSION=1.1.1
DOWNLOAD=https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf

# Install buf based on goos
if [ $1 eq "windows" ]; then
  DOWNLOAD="${DOWNLOAD}-Windows-x86_64.exe"
  curl -sSL --fail "${DOWNLOAD}" -o /tmp/buf
elif [ $1 eq "darwin" ]; then
  DOWNLOAD="${DOWNLOAD}-Darwin-x86_64"
  curl -sSL --fail "${DOWNLOAD}" -o /tmp/buf
else
    DOWNLOAD="${DOWNLOAD}-Linux-x86_64"
    curl -sSL --fail "${DOWNLOAD}" -o /tmp/buf
fi

# Simple smoke test to ensure buf is installed
buf --version