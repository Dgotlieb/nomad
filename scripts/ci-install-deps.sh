#!/bin/bash

echo "Installing dependencies for goos:$1 goarch:$2"

#### Install buf CLI ####

VERSION=1.1.1
DOWNLOAD=https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf

echo "HOME- $HOME"

if [ $1 = "windows" ]; then
  DOWNLOAD="${DOWNLOAD}-Windows-x86_64.exe"
  # In GHA, $HOME evaluates to /home/runner
  wget --quiet "${DOWNLOAD}" -O "$HOME/.local/bin/buf.exe"
  chmod +x "$HOME/.local/bin/buf.exe"
elif [ $1 = "darwin" ]; then
  DOWNLOAD="${DOWNLOAD}-Darwin-x86_64.tar.gz"
  wget --quiet "${DOWNLOAD}" -O - | tar -xz -C /tmp
  # In GHA, $HOME evaluates to /home/runner
  mv /tmp/buf/bin/buf "$HOME/.local/bin"
  chmod +x "$HOME/.local/bin/buf"
  # Exit script; nothing more to do for darwin builds
  exit 0
else
  DOWNLOAD="${DOWNLOAD}-Linux-x86_64.tar.gz"
  wget --quiet "${DOWNLOAD}" -O - | tar -xz -C /tmp
  # In GHA, $HOME evaluates to /home/runner
  mv /tmp/buf/bin/buf "$HOME/.local/bin"
  chmod +x "$HOME/.local/bin/buf"
fi

# #### Install required libraries ####

# export DEBIAN_FRONTEND=noninteractive

# # Update and ensure we have apt-add-repository
# apt-get update
# apt-get install -y software-properties-common

# # Add i386 architecture (for libraries)
# dpkg --add-architecture i386

# # Update with i386, Go and Docker
# apt-get update

# # Install Core build utilities for Linux
# apt-get install -y \
# 	build-essential \
# 	git \
# 	libc6-dev-i386 \
# 	libpcre3-dev \
# 	linux-libc-dev:i386 \
# 	pkg-config \
# 	zip \
# 	curl \
# 	jq \
# 	tree \
# 	unzip \
# 	wget

# # Install 32 bit headers and libraries for linux/386 builds
# if [ $1 == "linux" && $2 == "386" ]; then
#     cho "Installing linux/386 dependencies"
#     apt-get install gcc-multilib 
# fi

# # Install ARM build utilities for arm builds
# if [ $2 == "arm" || $2 == "arm64" ]; then
#     echo "Installing arm/arm64 dependencies"
#     apt-get install -y \
#         binutils-aarch64-linux-gnu \
#         binutils-arm-linux-gnueabihf \
#         gcc-5-aarch64-linux-gnu \
#         gcc-arm-linux-gnueabi \
#         gcc-arm-linux-gnueabihf
# fi

# # Install Windows build utilities for windows builds
# if [ $1 == "windows" ]; then
#     echo "Installing windows dependencies"
#     apt-get install -y \
#         binutils-mingw-w64 \
#         gcc-mingw-w64
# fi

# # Ensure everything is up to date
# apt-get upgrade -y
