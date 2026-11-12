# Security policy

## Supported versions

Only the latest release gets security fixes. Pre-releases (`-rc` versions) are replaced by the next release.

## Reporting a vulnerability

Please don't open a public issue. Report it privately through
[GitHub's private vulnerability reporting](https://github.com/akfaiz/carryover/security/advisories/new) instead,
with the steps to reproduce it and the version you tested.

You should get an answer within a week. Once a fix is released, the advisory is published with credit to you,
unless you'd rather stay anonymous.

Carryover keeps your rclone configuration, which holds the access tokens for your cloud storage, in its data folder
(`%AppData%\Carryover` on Windows, `~/.config/Carryover` on Linux). Anything that could expose that file, or let
someone else's library or cloud data run code or write outside the save folders, counts as a vulnerability.
