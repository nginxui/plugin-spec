# RFC 0012: Embedded package signatures and key derived trust

| | |
| --- | --- |
| Status | Accepted |
| Date | 2026-09-26 |
| Spec changes | PKG-13 (amended), PKG-14 (`signature_url` removed), PKG-16 (rewritten), PKG-19 through PKG-24, SEC-12 (rewritten), SEC-18 through SEC-24, CONF-1, the lint table of `spec/09-conformance.md`, `schema/catalog.schema.json` (`signature_url` and `signed_by` removed, `author_public_key` and `trust` described), the layout table of `README.md`, a note in RFC 0001 |
| Supersedes | The detached `.minisig` signatures of RFC 0001 |
| Reference host | nginx-ui `internal/pkgsign`, `internal/releasesign` (partner keys), `internal/plugin` (package verification, marketplace, offline packages, cluster sync, lint), `model/plugin.go` (`Trust`, `Signer`), `settings/plugin.go` (`DeveloperMode` replaces `RequireSignature`), `app/src/views/system/plugins` |

## Summary

A package carries its own signature. `plugin.sums` at the package root lists
the SHA-256 of every other file in the layout `sha256sum` prints, and
`plugin.sums.minisig` is a minisign signature of that list. The detached
`.minisig` next to an archive and the catalog members `signature_url` and
`signed_by` are gone; `sha256` stays as a check of the download. A host
derives the trust level of a package from the key that signed it: a release
key of the project pinned in the host binary gives `official`, a pinned
partner key `verified`, the author key of the catalog entry or a key the
operator trusts `community`, and anything else `unsigned`. The catalog
`trust` member becomes a label. A host installs an unsigned package only in
developer mode, which replaces `require_signature`, updates automatically
only official and verified plugins, and never lets an update lower the
trust of an installed plugin.

## Motivation

The first design signed the archive with a detached minisign signature and
let the catalog say how far to trust it. Four problems followed.

1. **The signature did not travel with the package.** It lived at
   `signature_url` in a catalog, as a sibling `.minisig` in the offline
   package directory and next to the archive in the cluster cache. Every
   path that moved a package had to move a second file. An upload through
   the UI or the CLI and a package a controller pushed to a node carried
   only the archive, so the reference host installed both without any
   signature check.
2. **Trust came from the catalog.** The level a host showed and enforced was
   the `trust` member of the catalog entry, honored when the source was the
   official catalog or `require_signature` was on, and `signed_by` told the
   host which keys to try. A level was only as good as the catalog that
   claimed it, and a package installed by upload had no level at all.
3. **`verified` promised a review.** It meant the maintainers had read the
   source before signing it with a release key. That is a claim the project
   cannot keep for every release of every plugin, and it made a maintainer's
   review part of the security model.
4. **Whether a signature was required depended on the path.** The official
   catalog always required one, a custom catalog only with
   `require_signature`, and the offline package directory and uploads
   followed rules of their own.

## Design

### Package format

The two files sit at the package root, the directory that holds
`plugin.json` (PKG-2):

```text
com.example.plugin/
├── CHANGELOG.md
├── LICENSE
├── README.md
├── plugin.json
├── plugin.sums
├── plugin.sums.minisig
└── server/
    └── plugin-linux-amd64
```

`plugin.sums` (PKG-19) lists every regular file except those two, sorted by
path in byte order, with `<sha256>` standing for 64 lowercase hexadecimal
digits:

```text
<sha256>  CHANGELOG.md
<sha256>  LICENSE
<sha256>  README.md
<sha256>  plugin.json
<sha256>  server/plugin-linux-amd64
```

`plugin.sums.minisig` (PKG-20) is what `minisign -S -m plugin.sums` writes.
Hosts accept both algorithms: current minisign releases write the prehashed
`ED` form by default and `minisign -l` writes the legacy `Ed` form.

With GNU coreutils a publisher signs and packs a per-platform package like
this (PKG-23):

```sh
cd com.example.plugin
find . -type f ! -path ./plugin.sums ! -path ./plugin.sums.minisig -printf '%P\0' \
  | LC_ALL=C sort -z | xargs -0 sha256sum > plugin.sums
minisign -S -s ~/.minisign/minisign.key -m plugin.sums
cd .. && tar czf com.example.plugin-1.0.0-linux-amd64.tar.gz com.example.plugin
```

Anyone can check an extracted package by hand with `sha256sum -c
plugin.sums` and `minisign -V -p author.pub -m plugin.sums`; a host also
catches a file that `plugin.sums` leaves out.

The signature covers what a host installs, the extracted files, rather than
the bytes of one archive. A package keeps its signature through a repack,
another compression level or a top level directory added or removed (PKG-2),
and the host checks the files it is about to run instead of a container it
has already unpacked.

### Signature state

A package is unsigned when it lacks either file or when no key the host
knows verifies the signature, signed when a known key verifies it and
`plugin.sums` matches the files, and invalid when a known key verifies it
but the files disagree with `plugin.sums` (PKG-21). The asymmetry is
deliberate. An unknown signer is not an error: the package may be sound and
only its key unknown to this host. A list that a trusted key signed and the
files contradict proves the package changed after signing, so a host refuses
it in every mode.

### Trust derivation

| Signer | Level |
| --- | --- |
| A release key of the Nginx UI project, pinned in the host binary | `official` |
| A partner key, pinned in the host binary | `verified` |
| The `author_public_key` of the catalog entry the package was downloaded from, or a key on the operator's trusted key list | `community` |
| No signature, or an unknown signer | `unsigned` |

The levels rank `unsigned` < `community` < `verified` < `official`, and a key
listed in several rows gives its highest level (SEC-18). Only the pinned key
sets reach `verified` or `official`, so neither a catalog nor an operator can
raise a key above `community`. `verified` is reserved for partner
organizations of the project whose keys the project pins; it names the
publisher and says nothing about a review.

A host derives the level when it inspects a package for a person and again
when it installs it, for every source (PKG-22). The catalog `trust` member
is only a label for the marketplace list and its filters (SEC-19). A package
without a catalog entry, from an upload, the offline package directory or a
cluster push, has no `author_public_key` to match and is `community` only
when the operator trusts its key.

### Host policy

| Situation | Rule |
| --- | --- |
| Unsigned package, developer mode off | Refused from every source (SEC-20) |
| Unsigned package, developer mode on | Installed and shown as unsigned (SEC-20) |
| Invalid package | Refused, in developer mode too (PKG-21) |
| Community package | Needs the community policy and a person's confirmation (SEC-21) |
| Automatic update | Only for plugins installed as `official` or `verified` (SEC-22) |
| Update whose level ranks below the installed one | Refused, whoever started it (SEC-22) |
| Automatic install of the DNS-01 plugin | `official` packages only (SEC-23) |

A host records the derived level and the signer key id of every installed
plugin and shows both (SEC-24). In the reference host `plugin.developer_mode`,
off by default, replaces `plugin.require_signature`, and
`plugin.trusted_public_keys` stays the operator's key list, whose keys now
give `community`.

### Catalog

`signature_url`, at the release and at the download level, and `signed_by`
are removed. `sha256` stays: it rejects a truncated or swapped download
before a host spends time extracting it, but it comes from the same document
as the URL and proves nothing about the publisher (PKG-16).
`author_public_key` stays and is the only way a catalog affects trust, for
the packages of its own entry and never above `community` (PKG-24). `trust`
stays as a label.

### Tooling

`nginx-ui plugin lint` reports a present `plugin.sums` that does not match
the files as an error, signed or not, warns when only one of the two files
is present, and warns when the signature does not verify with the keys
pinned in its binary (the lint table of `spec/09-conformance.md`).
`nginx-ui plugin inspect` and the upload dialog report the derived level and
the signer key id. Nothing downloads, keeps or moves a `.minisig` any more:
the offline package directory, `nginx-ui plugin fetch` and the cluster cache
hold archives only.

## Compatibility

* The plugin system has not shipped in an nginx-ui release (RFC 0001 made
  the same observation), so no deployed host or catalog depends on the
  detached signature. Removing `signature_url` and `signed_by` and requiring
  a signature outside developer mode is therefore made within spec 1.0
  rather than as a new major version (VER-1, VER-2).
* A catalog that still carries `signature_url` or `signed_by` stays valid
  against `schema/catalog.schema.json`, which allows unknown members, and a
  host ignores both (VER-4). A `.minisig` file next to an archive has no
  meaning any more.
* A package built before this RFC has no `plugin.sums` and is unsigned: it
  installs only in developer mode. Publishers rebuild and sign their
  packages.
* Uploads and cluster pushes, which were never verified before, now need a
  signed package or developer mode on the receiving host. An operator who
  installs their own unsigned builds turns developer mode on.

## What a publisher must adopt

1. Build the package tree, then write `plugin.sums` and sign it as the last
   step before packing (PKG-23). Sign every package of a release on its own,
   the portable one and each per-platform one.
2. Stop publishing `.minisig` files, and remove `signature_url` and
   `signed_by` from catalog releases; keep `sha256`.
3. A community author publishes the public key in the catalog entry's
   `author_public_key` and signs with the matching secret key.
4. The project signs official plugins with its release key in CI; a partner
   signs with the key the project pinned for it.

## Alternatives considered

* **The detached sidecar, the previous design.** Signing the archive bytes
  is simple, but the signature has to be carried separately along every
  path. The paths that carry only the archive, an upload and a cluster push,
  lose it exactly where a check matters, and the catalog needs members to
  locate it.
* **A signed catalog.** Signing the catalog document, or each release object
  with its `sha256`, proves what the catalog says, not what a package is. A
  package outside a catalog still proves nothing, trust stays a property of
  the catalog, every third party catalog needs a key of its own that hosts
  must learn, and the catalog maintainer rather than the publisher becomes
  the source of trust. The catalog also has to be signed again on every
  change.
* **A signature over the archive stored inside it**, as a last tar member or
  in a gzip or PAX header. The archive bytes cannot carry their own signature
  without a canonical form that leaves it out, stock `tar` and `gzip` produce
  none (PKG-10), and any repack breaks it. A list of file digests signs the
  content and ignores the container.
* **Digests in `plugin.json`.** The author writes the manifest and catalogs
  copy it (PKG-17); build output does not belong there, and the manifest
  would have to leave itself out.
* **Keeping `verified` as a reviewed level.** It ties a security claim to a
  manual process the project cannot scale. A partner key names a publisher,
  which is all a signature can prove.
* **Keeping `require_signature`.** A switch named after the check suggests
  that turning it off is harmless. Developer mode names the one situation it
  exists for, a developer installing their own unsigned build, and applies
  to every path the same way.

## Security considerations

* Outside developer mode, tampering always ends in a refusal. Changing a
  file without touching `plugin.sums` makes the package invalid; rewriting
  `plugin.sums` breaks the signature, so the signer is unknown and the
  package unsigned; removing both files leaves it unsigned. In developer
  mode a tampered package installs only as unsigned and is shown as such; it
  never keeps the level of its original signer.
* The signature covers the content and the path of every regular file. It
  does not cover permission bits, owners, timestamps or directories: a host
  sets executable bits itself (PKG-9), and an empty directory adds nothing a
  host reads.
* The path checks and extraction limits (PKG-3 through PKG-7) run before the
  signature is checked, so a hostile archive is bounded before it is
  trusted, and no file of a package runs before the check completes
  (PKG-22). Checking again at install time, after an inspect, keeps a file
  swapped between the two from inheriting the level the person saw.
* A catalog can make its own packages `community` through
  `author_public_key`, the level that always needs the community policy and
  a person's confirmation; it cannot reach `verified` or `official`. A
  compromised catalog that swaps `author_public_key` cannot replace an
  official or verified plugin, since the update would rank below the
  installed level (SEC-22), and automatic updates never touch community
  plugins.
* A minisign key id only selects the key. The host verifies the signature
  with the key itself, so a colliding or forged key id gains nothing.
* Removing a compromised pinned key needs a host release.

## Future work

* Pinning the signer key of an installed community plugin, so that an update
  signed by another community key asks the person again.
* A revocation list for pinned keys that does not wait for a host release.
* Catalog tooling that refuses to list a release whose packages are not
  signed with the key of its entry.
* The Go packer suggested in RFC 0001, writing `plugin.sums` and signing in
  one step.
