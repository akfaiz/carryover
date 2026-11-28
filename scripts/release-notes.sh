#!/usr/bin/env bash
# Prints a release's description: its section of CHANGELOG.md, with the hard-wrapped
# lines joined. GitHub shows every newline of a release body as a line break, and the
# app's updater shows the body as it is.
#
#   scripts/release-notes.sh <tag> [changelog]      e.g. scripts/release-notes.sh v0.2.0
#
# A pre-release tag (v0.2.0-beta.1) without a section of its own gets [Unreleased].
# A tag with nothing to show is an error.
set -euo pipefail

tag="${1:?usage: release-notes.sh <tag> [changelog]}"
changelog="${2:-$(dirname "$0")/../CHANGELOG.md}"
version="${tag#v}"

# section <name> prints what is under "## [<name>]", one line per paragraph or list item.
section() {
  awk -v heading="## [$1]" '
    function flush() { if (line != "") print line; line = "" }
    # Blank lines are printed late, so that none lead or trail the section.
    function gap() { if (blank && printed) print ""; blank = 0; printed = 1 }

    { sub(/\r$/, "") }
    /^## / { if (found) exit; found = (index($0, heading) == 1); next }
    !found { next }
    /^\[[^]]+\]:[ \t]/ { exit } # the link definitions after the last section

    /^[ \t]*```/ { flush(); gap(); print; code = !code; next }
    code { print; next }

    /^[ \t]*$/ { flush(); blank = 1; next }
    /^#+ / { flush(); gap(); print; next }
    /^[ \t]*([-*+]|[0-9]+\.) / || /^[ \t]*\|/ { flush(); gap(); line = $0; next }
    line == "" { gap(); line = $0; next }
    { sub(/^[ \t]+/, ""); line = line " " $0 }
    END { flush() }
  ' "$changelog"
}

notes=$(section "$version")
if [[ -z $notes && $version == *-* ]]; then
  notes=$(section Unreleased)
fi
if [[ -z $notes ]]; then
  echo "no notes for $tag in $changelog: add a \"## [$version]\" section" >&2
  exit 1
fi
printf '%s\n' "$notes"
