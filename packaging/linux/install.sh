#!/bin/sh
# Installs Carryover from this folder.
#
#   ./install.sh                    for your user, into ~/.local (no root needed)
#   sudo ./install.sh /usr/local    for every user
#
# The menu entry points at the installed binary by its full path, so it works
# even when <prefix>/bin is not on PATH. Keep the install where it is: Steam
# shortcuts made by "Add to Steam" point at that path too.
set -eu

cd "$(dirname "$0")"
prefix="${1:-$HOME/.local}"

install -Dm755 bin/carryover "$prefix/bin/carryover"
install -Dm644 share/icons/hicolor/scalable/apps/carryover.svg \
  "$prefix/share/icons/hicolor/scalable/apps/carryover.svg"
mkdir -p "$prefix/share/applications"
sed "s|^Exec=carryover\$|Exec=$prefix/bin/carryover|" share/applications/carryover.desktop \
  > "$prefix/share/applications/carryover.desktop"

echo "Installed Carryover to $prefix/bin/carryover"
echo "It is now in your applications menu (log out and back in if it doesn't show up)."
