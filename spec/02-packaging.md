# 02. Packaging

A plugin release is distributed as gzip-compressed tar archives: either one
portable package, or one package per platform, or both. This document
describes the archive format, the limits the reference host enforces while
extracting one (`internal/plugin/package.go`), the signature a package
carries inside itself (PKG-19 through PKG-23, introduced by
[RFC 0012](rfcs/0012-embedded-package-signatures.md)), the certificate a
partner package carries for the key that signed it (PKG-25 through PKG-27,
introduced by [RFC 0013](rfcs/0013-partner-certificates-and-keyring.md)),
and how a catalog release points a host at the package for its platform
(PKG-14 onwards, introduced by
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
`webapp.style_path`, `webapp.chunks` values, `webapp.pages[].file`, `content.templates`,
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
`plugin.json` generator (e.g. `cmd/manifest` in `plugin-dns01`)
achieves this for the manifest by encoding every map in key order.

## PKG-11

A host MUST validate the manifest (`spec/01-manifest.md`) as part of
extraction, before treating the plugin as installed, and MUST discard the
extracted directory entirely if validation fails.

## Embedded signature

A package proves who published it with two files at its package root:
`plugin.sums`, which lists the SHA-256 of every other file, and
`plugin.sums.minisig`, a [minisign](https://jedisct1.github.io/minisign/)
signature of that list. The signature travels inside the archive, so a host
checks it the same way whether the package came from a catalog, an upload,
the offline package directory or another host of a cluster. The **package
root** is the directory that holds `plugin.json` after PKG-2, so a package
signs the same whether or not its entries sit under one top level directory.
The trust level the key of a signer earns is specified in
`spec/08-security.md` (SEC-18).

## PKG-19: `plugin.sums`

`plugin.sums` is a text file at the package root with one line for every
regular file of the package, except `plugin.sums` and `plugin.sums.minisig`
at the package root themselves:

```text
<sha256>  <path>
```

* `<sha256>` is the SHA-256 of the file content as 64 lowercase hexadecimal
  digits;
* two spaces separate it from `<path>`, the path of the file relative to the
  package root with `/` as the separator, which MUST be a safe relative path
  (PKG-3);
* every line, the last one included, ends with a single LF (`0x0A`); the
  file has no CR, no empty line, no byte order mark and no comment;
* lines are sorted by path in ascending byte order (the order
  `LC_ALL=C sort` produces), and no path appears twice;
* directories are not listed; every regular file is, an empty one included.

This is the layout `sha256sum` prints and `sha256sum -c` reads. A file named
`plugin.sums` or `plugin.sums.minisig` below the package root is an ordinary
file and is listed. The certificate files `plugin.partner` and
`plugin.partner.minisig` (PKG-25, PKG-26) are ordinary files too and are
listed, at the package root as anywhere else. A path that contains a LF or CR byte cannot be listed,
so a package that carries `plugin.sums` MUST NOT contain such a file.

## PKG-20: `plugin.sums.minisig`

`plugin.sums.minisig` is a minisign signature of the exact bytes of
`plugin.sums`, in the text form `minisign -S -m plugin.sums` writes: an
untrusted comment line, the signature, a trusted comment line and the
signature of the trusted comment. A host MUST accept both minisign
algorithms, the legacy `Ed` signature of the file itself and the prehashed
`ED` signature of its BLAKE2b-512 digest, and MUST verify a signature the
way `minisign -V` does, the signature of the trusted comment included. Both
comments are free text and carry no meaning in this spec.

## PKG-21: signature state

A host determines the signature state of a package from its extracted
files:

| The package holds | State |
| --- | --- |
| Neither `plugin.sums` nor `plugin.sums.minisig`, or only one of them | Unsigned |
| Both, and the signature names a key id the host does not know (SEC-18) | Unsigned: an unknown signer counts as no signer |
| Both, and the signature does not parse, or names a key the host knows that does not verify it | Invalid |
| Both, a key the host knows verifies the signature, and `plugin.sums` matches the files | Signed by that key |
| Both, a key the host knows verifies the signature, and `plugin.sums` does not match the files | Invalid |

`plugin.sums` matches the files when it follows PKG-19 exactly and lists
every regular file of the package, and nothing else, with the SHA-256 of its
content. A differing digest, a listed path that is missing or is not a
regular file, a regular file that is not listed and a line that breaks
PKG-19 are each a mismatch.

The keys a host knows are the keys SEC-18 gives a level: its release keys,
a partner key that a certificate in the package (PKG-27) or the partner
keyring (SEC-25) vouches for and that is not revoked (SEC-28), the
`author_public_key` of the catalog entry the package was downloaded from and
the keys on the host's trusted key list. A key that only a certificate
failing PKG-27 names is unknown, so a failed certificate never makes a
package invalid by itself.

A host MUST refuse an invalid package in every mode, developer mode included
(SEC-20): either a key it trusts signed a list the files no longer agree
with, or the signature is damaged or claims a key it trusts and fails, so
the package changed after it was signed. What a host does with an unsigned
package is specified by SEC-20. A host SHOULD tell the person which row
applied, so that a missing signature, a half signed package and an unknown
signer can be told apart.

## PKG-22: when a host checks

A host MUST determine the signature state and the trust level (SEC-18) of a
package:

* when it inspects the package for a person before installing it (for
  example an upload dialog or `nginx-ui plugin inspect`), so the person sees
  the level before deciding; and
* again when it installs the package, whatever it found while inspecting.

This applies to every source a package reaches a host from: an upload, the
offline package directory, a catalog download (next to the SHA-256 check of
PKG-16) and a push from another host (PKG-18). The check covers the files as
the host extracts them, after the checks of PKG-2 through PKG-7, and MUST be
complete before the host runs any file of the package or replaces an
installed plugin with it. A host MUST NOT skip it because a digest matched
or because another host already checked the same package.

The partner certificate (PKG-27) and the partner keyring (SEC-25) are part
of the check. A host evaluates them with the keyring it holds and the date
of its clock at that moment (SEC-29), so the level a package derives when it
is installed can differ from the one the person saw when it was inspected.

## PKG-23: signing a package

A publisher signs a package once every other file of it is final: it writes
`plugin.sums` (PKG-19), then runs `minisign -S -m plugin.sums` with its
secret key. Any later change to a file of the package needs a new
`plugin.sums` and a new signature. Every package of a release is signed on
its own, since each carries its own `plugin.json` and executables (PKG-12,
PKG-13). A package offered in a catalog SHOULD be signed: outside developer
mode no host installs an unsigned package (SEC-20).

A partner copies the two files of its certificate (PKG-25, PKG-26) unchanged
into the package root before it writes `plugin.sums`, so that `plugin.sums`
lists them. A certificate belongs to a key, not to a plugin: one certificate
serves every package the partner signs with that key. A renewed certificate
is a changed file and needs a new `plugin.sums` and a new signature.

## Partner certificate

A partner of the Nginx UI project signs its packages with a key of its own.
So that such a package derives `verified` on a host that has never heard of
the partner, the package carries a certificate for that key:
`plugin.partner`, the public key of the partner, and
`plugin.partner.minisig`, a signature of it by a release key of the project
whose trusted comment names the partner and, optionally, the last day the
certificate is valid. A host needs nothing but its release keys to check a certificate, so
a certificate works offline and a new partner needs no release of the host.
The level a certificate earns, the partner keyring that lists partner keys
without a certificate, and the revocation and expiry of a partner key are
specified in `spec/08-security.md` (SEC-18, SEC-25 through SEC-29).

## PKG-25: `plugin.partner`

`plugin.partner` is a text file at the package root that holds the minisign
public key of the partner that signs the package, in the form of a minisign
`.pub` file:

* an untrusted comment line that starts with `untrusted comment: `, whose
  text carries no meaning in this spec;
* the key line, the base64 encoding minisign writes of the algorithm `Ed`,
  the key id and the Ed25519 public key.

A file that holds the key line alone is also accepted. Every line ends with
a LF (`0x0A`), which the last line MAY omit. A file named `plugin.partner`
below the package root is an ordinary file and no certificate.

## PKG-26: `plugin.partner.minisig`

`plugin.partner.minisig` is a minisign signature of the exact bytes of
`plugin.partner` by a release key of the Nginx UI project (SEC-18), in the
text form PKG-20 describes. The algorithms and the verification of PKG-20
apply, the signature of the trusted comment included. The trusted comment,
the text that follows `trusted comment: ` on its line, MUST be exactly one
of:

```text
partner:<name>
partner:<name>;expires:<YYYY-MM-DD>
```

* `<name>` names the partner: one or more ASCII letters, digits, dots and
  hyphens (`^[A-Za-z0-9.-]+$`);
* `;expires:<YYYY-MM-DD>` is optional. `<YYYY-MM-DD>` is the last day the
  certificate is valid, a date of the UTC calendar; the certificate is valid
  through the whole of that day (SEC-29). A certificate without it does not
  expire and stays valid until the keyring revokes its key (SEC-28);
* nothing else: no space, no other field and no other order.

minisign signs the trusted comment together with the signature, so the name
and the date are as authentic as the key. The untrusted comment of the
signature carries no meaning. The project issues a certificate with:

```sh
minisign -S -s <release key> -m plugin.partner -t "partner:<name>;expires:<YYYY-MM-DD>"
```

and SHOULD give it an expiry date (SEC-29).

## PKG-27: checking a certificate

A package carries a certificate when both `plugin.partner` and
`plugin.partner.minisig` are at its package root. Whenever a host determines
the signature state and the trust level of such a package (PKG-22), it MUST
check the certificate in this order, and the certificate passes only when
every step does:

1. `plugin.partner` parses as a minisign public key (PKG-25);
2. `plugin.partner.minisig` parses, and one of the release keys of the host
   verifies it over the exact bytes of `plugin.partner`, the trusted comment
   included (PKG-26);
3. the trusted comment follows PKG-26;
4. when the trusted comment carries an expiry date, the certificate has not
   expired by the date of the host's clock (SEC-29);
5. the partner keyring the host holds, if any, does not list the key id of
   the partner key in `revoked` (SEC-28).

A certificate that passes makes the partner key a key the host knows
(PKG-21). When that key verifies `plugin.sums.minisig` and `plugin.sums`
matches the files, the package is signed by the partner key and derives
`verified` (SEC-18). Since `plugin.sums` lists the certificate (PKG-19), a
package whose certificate was added, replaced or removed after signing no
longer matches its `plugin.sums`.

A certificate that fails a step, and a package that holds only one of the
two files, gives no partner trust. The host ignores the certificate and
derives the level of the package from its remaining sources: a keyring
entry, the catalog entry or the operator's trusted key list (SEC-18). A
certificate that fails MUST NOT make a package invalid by itself (PKG-21). A
host SHOULD tell the person which step failed, so that an expired
certificate, a revoked key and a certificate the project never issued can be
told apart.

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
MUST ship the same files apart from the executables themselves and from
`plugin.sums` and `plugin.sums.minisig`, which list and sign them (PKG-19,
PKG-20). The permissions, dependencies, settings and webapp a person
approves from the catalog's manifest snapshot (PKG-17) are then the ones the
host installs, whichever package it picked.

## Distribution catalog

A catalog is a static JSON document a host fetches to offer plugins for
installation. [`schema/catalog.schema.json`](../schema/catalog.schema.json)
describes its full shape (`schema_version` 1); the requirements below cover
how a release names its packages, how a host chooses one and what an entry
says about its publisher.

## PKG-14

A catalog release MAY carry `downloads`, an object keyed by a platform key
(`<goos>-<goarch>`, NAME-9) or by `any`, whose values describe one package
each:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `url` | string | yes | Where the package is downloaded from. |
| `sha256` | string | no, SHOULD be present | Lowercase hex SHA-256 of the package file, a check of the download (PKG-16). |

The release-level `download_url` and `sha256` describe the portable package
and stay the fallback for every platform `downloads` does not name.
`download_url` MAY be omitted or empty when `downloads` is present; a
release MUST have at least one of the two. A `downloads` entry whose key is
a platform MUST point at a per-platform package of that platform (PKG-12); an
`any` entry MUST point at a package that runs everywhere (no `server`, or an
interpreted `server.command`).

`platforms` stays the summary of where the release installs: it MUST list
every key of `downloads` plus every platform the portable package serves. An
absent or empty `platforms` keeps its original meaning, the portable package
runs on every platform.

A release names no signature: the signature is inside each package (PKG-19,
PKG-20). The `signature_url` and `signed_by` members of catalogs written
before [RFC 0012](rfcs/0012-embedded-package-signatures.md) have no meaning,
and a host ignores them like any other member it does not know.

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
the selected entry declares, MUST determine the signature state and the
trust level of the package (PKG-21, PKG-22, SEC-18) exactly as for a
package from any other source, and after extraction MUST check that the
manifest `id` and `version` equal the catalog entry id and the release
version. The digest comes from the same catalog as the URL: it shows that
the download is the file the catalog meant, not who published it, so a
matching digest never replaces the signature check. A host MUST refuse to
install a package that has no executable for its own platform and no
`server.command` to fall back to (MAN-14), whatever the catalog claimed.

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

## PKG-24

A catalog entry MAY carry two members about who publishes the plugin:

| Field | Type | Meaning |
| --- | --- | --- |
| `author_public_key` | string | The minisign public key the author signs the plugin's packages with, the base64 line of a minisign `.pub` file. A package downloaded from this entry and signed with this key is `community` (SEC-18). |
| `trust` | string | `official`, `verified` or `community`: the level the catalog expects the packages of the entry to derive. |

`author_public_key` counts only for the packages a host downloads from the
entry that carries it, never for a package of another entry or one that
reached the host another way. `trust` is a label for listing and filtering
entries before anything is downloaded; a host MUST NOT grant a level because
of it (SEC-19).

## PKG-28

A release is beta when its `version` has a prerelease part (semantic
versioning 2.0.0, for example `1.0.0-beta.1` or `2.0.0-rc.1`), or when its
catalog release carries `"beta": true`. The member is optional and is for a
publisher who ships a plain version but still calls the release beta. An
entry whose `stage` is `beta` is beta as a whole.

A host SHOULD show a beta release, and an installed plugin whose version has
a prerelease part, with a mark that says it is still being tested. A host
SHOULD NOT move a stable installation to a beta release on its own: when it
picks the newest installable release (PKG-15), reports available updates or
updates automatically, it SHOULD choose among the stable releases. It MAY
choose a beta release when the installed version is beta itself, or when
nothing is installed and the catalog has no stable release that installs.
Installing a beta release the person asked for by version is always allowed.
