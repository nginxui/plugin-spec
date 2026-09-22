# 02. Packaging

A plugin release is distributed as gzip-compressed tar archives: either one
portable package, or one package per platform, or both. This document
describes the archive format, the limits the reference host enforces while
extracting one (`internal/plugin/package.go`), and how a catalog release
points a host at the package for its platform (PKG-14 onwards, introduced by
[RFC 0001](rfcs/0001-per-platform-packages.md)).

## PKG-1

A plugin package MUST be a gzip-compressed tar archive (`.tar.gz`) named
either `<id>-<version>.tar.gz` (a **portable** package) or
`<id>-<version>-<goos>-<goarch>.tar.gz` (a **per-platform** package, PKG-12),
where `<id>` is the manifest's `id`, `<version>` is its `version` and
`<goos>-<goarch>` is a platform key as defined by NAME-9. A host MUST NOT rely
on the file name for anything it can read from the manifest: an uploaded
package may arrive under any name.

## PKG-2

`plugin.json` MUST be readable at the root of the archive after extraction.
A host MAY additionally accept an archive whose entries are all nested under
a single top level directory (the layout `tar czf x.tar.gz plugin-dir/`
produces), in which case the host strips that one leading directory before
looking for `plugin.json` at the resulting root. A host MUST reject an
archive where `plugin.json` is neither at the root nor exactly one directory
down.

## PKG-3: safe relative paths

Every path a manifest gives (`icon_path`, `server.executables[*]`,
`server.command[0]` when it contains a separator, `webapp.bundle_path`,
`webapp.style_path`, `webapp.pages[].file`, `content.templates`,
`content.locales`) and every archive entry name MUST be a **safe relative
path**:

* it MUST use `/` as the separator, never `\`;
* it MUST NOT be empty and MUST NOT contain a NUL byte;
* it MUST NOT be absolute (neither `/foo` nor a Windows drive/share prefix
  such as `C:foo`);
* it MUST be [`path.Clean`](https://pkg.go.dev/path#Clean)-equivalent already
  (no `.` or redundant `//` segments to normalize away);
* it MUST NOT be `.` or `..`, and MUST NOT start with `../`.

A host MUST reject a manifest or an archive entry that fails any of these
checks rather than silently normalizing it.

## PKG-4

An archive entry MUST NOT be a symbolic link or a hard link. A host MUST
reject the whole package if any entry's type is a link.

## PKG-5

Extracting a package MUST NOT be able to write outside the destination
directory. A host MUST re-verify, after resolving `..` and symlinks in the
destination path itself, that every extracted path stays inside the
destination directory, in addition to the name-level check in PKG-3.

## PKG-6

A package MUST contain no more than **10,000** entries (files and
directories combined). A host MUST stop extracting and reject the package as
soon as this limit is exceeded, without waiting for the rest of the archive.

## PKG-7

The total uncompressed size of a package's regular files MUST NOT exceed
**256 MiB** (`256 * 1024 * 1024` bytes). A host MUST stop extracting and
reject the package as soon as this limit is exceeded.

## PKG-8

A package MUST include, at its root, `README.md`, `LICENSE` and
`CHANGELOG.md`. These are documentation for the person installing the
plugin and for whoever audits it; a host SHOULD refuse to install a package
missing one of them, and MUST make their absence visible to the installer
when it does not refuse.

## PKG-9

A host MUST make every file that a resolved `server.executables[*]` entry or
a path-containing `server.command[0]` points at executable (mode `0755` or
equivalent) after extraction, regardless of the permission bits stored in the
archive, and MUST leave every other extracted regular file's executable bit
untouched.

A package MUST contain every file its `server.executables` declares. A
platform a package does not ship is left out of `server.executables` rather
than declared without its file (see PKG-12); the reference linter reports a
declared but missing executable under this id.

## PKG-10

Regenerating a package from the same source tree SHOULD be
byte-for-byte deterministic wherever the toolchain allows it (stable file
order, no embedded timestamps beyond what the archive format requires), so
that a package's checksum is a meaningful integrity signal. The reference
`plugin.json` generator (e.g. `cmd/manifest` in `nginx-ui-plugin-dns01`)
achieves this for the manifest by encoding every map in key order.

## PKG-11

A host MUST validate the manifest (`spec/01-manifest.md`) as part of
extraction, before treating the plugin as installed, and MUST discard the
extracted directory entirely if validation fails.

## Per-platform packages

A native plugin ships one executable per platform. Putting every executable
into one archive makes each node download all of them and quickly exceeds the
PKG-7 budget, so a release MAY instead ship one package per platform.

## PKG-12

A per-platform package, one whose file name carries a `-<goos>-<goarch>`
suffix (PKG-1), MUST declare exactly that platform in `server.executables`:
the map has one key, and that key equals the suffix. Together with PKG-9 this
means the package ships exactly one executable, the one for the platform its
name promises. A portable package MAY declare any number of platforms, and
MUST ship every one it declares (PKG-9).

## PKG-13

All packages of one release (the portable package and every per-platform
package) MUST carry the same manifest apart from `server.executables` and
MUST ship the same files apart from the executables themselves. The
permissions, dependencies, settings and webapp a person approves from the
catalog's manifest snapshot (PKG-17) are then the ones the host installs,
whichever package it picked.

## Distribution catalog

A catalog is a static JSON document a host fetches to offer plugins for
installation. [`schema/catalog.schema.json`](../schema/catalog.schema.json)
describes its full shape (`schema_version` 1); the requirements below cover
how a release names its packages and how a host chooses one.

## PKG-14

A catalog release MAY carry `downloads`, an object keyed by a platform key
(`<goos>-<goarch>`, NAME-9) or by `any`, whose values describe one package
each:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `url` | string | yes | Where the package is downloaded from. |
| `sha256` | string | no, SHOULD be present | Lowercase hex SHA-256 of the package file. |
| `signature_url` | string | no | Detached minisign signature of the package; defaults to `url` + `.minisig`. |

The release-level `download_url`, `sha256` and `signature_url` describe the
portable package and stay the fallback for every platform `downloads` does
not name. `download_url` MAY be omitted or empty when `downloads` is present;
a release MUST have at least one of the two. A `downloads` entry whose key is
a platform MUST point at a per-platform package of that platform (PKG-12); an
`any` entry MUST point at a package that runs everywhere (no `server`, or an
interpreted `server.command`).

`platforms` stays the summary of where the release installs: it MUST list
every key of `downloads` plus every platform the portable package serves. An
absent or empty `platforms` keeps its original meaning, the portable package
runs on every platform.

## PKG-15

A host running on platform `P` MUST select the package of a release as
follows, and MUST treat the release as not installable on `P` when no step
matches:

1. `downloads[P]`, when present with a non-empty `url`;
2. otherwise `downloads["any"]`, when present with a non-empty `url`;
3. otherwise the portable `download_url`, when it is non-empty and
   `platforms` is empty or contains `P` or `any`.

A host MUST use this selection wherever it decides what a release means for a
platform: when it picks the newest installable release, reports available
updates, resolves a dependency, installs a plugin without being asked (for
example a plugin a feature depends on), and when it downloads a package for
another node or for an offline install.

## PKG-16

Verification applies to the selected package: a host MUST check the SHA-256
the selected entry declares, MUST verify the signature at the selected
entry's signature URL under the same signature policy it applies to a
portable package, and after extraction MUST check that the manifest `id` and
`version` equal the catalog entry id and the release version. A host MUST
refuse to install a package that has no executable for its own platform and
no `server.command` to fall back to (MAN-14), whatever the catalog claimed.

## PKG-17

A catalog release's `manifest` snapshot MUST be the manifest of the release
as a whole: its `server.executables` lists every platform the release ships a
package for, even though each per-platform package only declares its own
(PKG-12). A host MAY read permissions, dependencies and the platform list
from the snapshot before it selects a package.

## PKG-18

A host that installs a plugin on another host (for example a controller
pushing a plugin to the nodes of a cluster) MUST NOT push a package that has
no executable for the receiving host's platform. It SHOULD let the receiving
host select its own package from the catalog first, and otherwise SHOULD
resolve a package for the receiving host's platform: a package it already
holds, or the catalog package PKG-15 selects for that platform. When none
exists it MUST report the platform as unsupported instead of pushing its own
package.
