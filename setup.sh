#!/bin/bash

# Grove Installation Script
# This script downloads and installs the latest version of Grove

set -e

# Cleanup function that runs on exit
cleanup() {
  if [ -d "/tmp/grove-install" ]; then
    echo "Cleaning up..."
    rm -rf /tmp/grove-install
  fi
}

# Ensure cleanup runs on exit (success or failure)
trap cleanup EXIT

echo "Installing Grove..."

# Create temporary directory
mkdir -p /tmp/grove-install

# Download latest release
echo "Downloading latest release..."
curl -s https://api.github.com/repos/Jordan-Kowal/grove/releases/latest | \
  grep "browser_download_url.*zip" | \
  cut -d '"' -f 4 | \
  xargs curl -L -o /tmp/grove-install/grove.zip

# Extract the app
echo "Extracting application..."
unzip -q /tmp/grove-install/grove.zip -d /tmp/grove-install

# Remove quarantine attributes
echo "Removing quarantine attributes..."
xattr -cr /tmp/grove-install/*.app

# Backup existing installation
BACKUP_PATH=""
if [ -d "/Applications/Grove.app" ]; then
  echo "Backing up existing installation..."
  BACKUP_PATH="/Applications/Grove.app.backup"
  mv /Applications/Grove.app "$BACKUP_PATH"
fi

# Install to Applications
echo "Installing to Applications folder..."
if mv /tmp/grove-install/*.app /Applications/; then
  # Installation successful, remove backup
  if [ -n "$BACKUP_PATH" ] && [ -d "$BACKUP_PATH" ]; then
    echo "Removing old version..."
    rm -rf "$BACKUP_PATH"
  fi
  echo "Grove has been successfully installed!"
else
  # Installation failed, restore backup
  echo "Installation failed!"
  if [ -n "$BACKUP_PATH" ] && [ -d "$BACKUP_PATH" ]; then
    echo "Restoring previous version..."
    mv "$BACKUP_PATH" /Applications/Grove.app
    echo "Previous version has been restored"
  fi
  exit 1
fi
