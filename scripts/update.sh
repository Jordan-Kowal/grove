#!/bin/bash
# Grove update script — downloads latest .dmg, installs, relaunches.
# Relies on Developer ID signing + notarization: no xattr or tccutil needed.

set -e

cleanup() {
  if [ -n "${MOUNT_POINT:-}" ] && [ -d "$MOUNT_POINT" ]; then
    hdiutil detach -quiet "$MOUNT_POINT" || true
  fi
  rm -rf /tmp/grove-install
}
trap cleanup EXIT

echo "Updating Grove..."

mkdir -p /tmp/grove-install

echo "Downloading latest release..."
curl -s https://api.github.com/repos/Jordan-Kowal/grove/releases/latest | \
  grep "browser_download_url.*\.dmg\"" | \
  head -1 | \
  cut -d '"' -f 4 | \
  xargs curl -L -o /tmp/grove-install/grove.dmg

echo "Mounting disk image..."
MOUNT_POINT=$(hdiutil attach -nobrowse -quiet -mountrandom /tmp /tmp/grove-install/grove.dmg | tail -1 | awk '{print $NF}')

echo "Installing to Applications folder..."
if [ -d "/Applications/Grove.app" ]; then
  rm -rf "/Applications/Grove.app"
fi
cp -R "$MOUNT_POINT/Grove.app" /Applications/

echo "Grove has been successfully updated!"
