# 02. Packaging

A plugin is distributed as a single gzip-compressed tar archive. This
document describes the archive format and the limits the reference host
enforces while extracting one (`internal/plugin/package.go`).

## PKG-1

A plugin package MUST be a gzip-compressed tar archive (`.tar.gz`) named
`<id>-<version>.tar.gz`, where `<id>` is the manifest's `id` and `<version>`
is its `version`.

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
