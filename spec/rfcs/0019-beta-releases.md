# RFC 0019: Beta releases

| | |
| --- | --- |
| Status | Accepted |
| Date | 2026-09-30 |
| Spec changes | PKG-28, the packaging level in `spec/09-conformance.md`, `beta` on a release in `schema/catalog.schema.json` |

## Summary

A host shows a release as beta when its version has a prerelease part or its
catalog release is marked `beta`, and does not move a stable installation to
a beta release on its own.

## Motivation

A plugin that is still being tested is published like any other, and the
marketplace gave no sign of it. Picking the highest version also made
`2.0.0-rc.1` the update of a stable `1.4.0`, so a person could end up on
unfinished software without choosing it.

## Design

* **What is beta.** A version with a prerelease part, read from the release
  version and not from a new manifest field. A publisher who ships a plain
  version but still calls it beta sets the optional `beta` member of the
  catalog release. The entry `stage` `beta` marks the whole plugin.
* **Display.** A host SHOULD mark beta releases in the marketplace and
  installed plugins whose version has a prerelease part.
* **Update selection.** A host SHOULD pick among stable releases for a first
  install and for a stable installation, and follow the newest release when
  the installed version is beta. With no stable release that installs, a
  first install may take a beta one. A stable installation is offered
  nothing. Installing a named version stays possible.

## Compatibility

A new optional catalog member (VER-1). Hosts that do not know it ignore it
and show no mark, and they keep choosing the highest version.
