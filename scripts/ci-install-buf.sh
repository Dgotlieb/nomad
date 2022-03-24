#!/bin/bash

echo "Installing dependencies for goos:$1"

# Make sure you grab the latest version
VERSION=1.1.1
DOWNLOAD=https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf

# In GHA, this evaluates to /home/runner/work/nomad/nomad/tmp
mkdir -p $PWD/tmp

# Install buf based on goos
if [ $1 = "windows" ]; then
  DOWNLOAD="${DOWNLOAD}-Windows-x86_64.exe"
  wget --quiet "${DOWNLOAD}" -O $PWD/tmp/buf.exe
  chmod +x $PWD/tmp/buf.exe
elif [ $1 = "darwin" ]; then
  DOWNLOAD="${DOWNLOAD}-Darwin-x86_64.tar.gz"
  wget --quiet "${DOWNLOAD}" -O - | tar -xz -C $PWD/tmp
  chmod +x $PWD/tmp/buf/bin/buf
else
  DOWNLOAD="${DOWNLOAD}-Linux-x86_64.tar.gz"
  wget --quiet "${DOWNLOAD}" -O - | tar -xz -C $PWD/tmp
  chmod +x $PWD/tmp/buf/bin/buf
fi
