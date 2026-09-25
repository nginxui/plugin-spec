# RFC 0001: Per-platform packages

| | |
| --- | --- |
| Status | Accepted, implemented; signature handling superseded by RFC 0012 |
| Date | 2026-09-23 |
| Spec changes | PKG-1 (amended), PKG-9 (clarified), PKG-12 through PKG-18, NAME-9, CONF-1 table, `schema/catalog.schema.json` |
| Reference host | nginx-ui `internal/plugin` (catalog selection, offline packages, cluster sync, lint), `app/src/views/system/plugins` |
| Reference plugin | nginx-ui-plugin-dns01 `build.sh`, `cmd/manifest -platform` |

The detached `.minisig` signatures, `signature_url` and `signed_by` that this
RFC mentions were replaced by the signature a package carries inside itself,
see [RFC 0012](0012-embedded-package-signatures.md). The text below is kept
as it was accepted.

## Summary

A plugin release may ship one package per platform,
`<id>-<version>-<goos>-<goarch>.tar.gz`, next to or instead of the portable
`<id>-<version>.tar.gz`. A per-platform package declares only its own
platform in `server.executables`. A catalog release lists the per-platform
packages in a new `downloads` map; `download_url` stays as the portable
fallback. A host selects `downloads[<its platform>]`, then `downloads["any"]`,
then the portable package, and applies that selection everywhere it chooses a
release. The catalog `schema_version` stays 1.

## Motivation

Spec 1.0 assumed one archive per release holding the executable of every
platform. The official DNS-01 plugin embeds the lego provider catalog and
compiles to 54 to 61 MiB per platform. Six platforms make:

| Artifact | Compressed | Unpacked |
| --- | --- | --- |
| One platform | 17 to 19 MiB | about 60 MiB |
| All six in one archive | 113 MiB | about 345 MiB |

The combined archive is over the 256 MiB unpacked budget every host enforces
(PKG-7), so it cannot be installed at all, and even under the budget every
node would download five binaries it never runs. A single-platform build
worked around it during development by declaring six platforms and shipping
one, which fails the linter (PKG-9) and leaves the catalog unable to say which
platforms a release really covers.

## Design

### Package forms and names

Two package forms exist (PKG-1, NAME-9):

| Form | File name | `server.executables` |
| --- | --- | --- |
| Portable | `<id>-<version>.tar.gz` | Any number of platforms, every one shipped; or none, for a plugin without `server` or with an interpreted `server.command`. |
| Per-platform | `<id>-<version>-<goos>-<goarch>.tar.gz` | Exactly the platform in the file name (PKG-12). |

Platform keys are Go's `GOOS` and `GOARCH` joined by a hyphen, the keys
`server.executables` already uses (MAN-14). `any` is reserved for "every
platform" in catalogs and never appears in a file name.

The linter keeps PKG-9 strict: every executable a package declares must be in
it. That is why a per-platform package narrows `server.executables` instead of
keeping the full map, and why PKG-12 is a separate rule: it catches a package
whose name promises one platform while its manifest declares another, or
several.

All packages of a release are identical apart from `server.executables` and
the binaries (PKG-13). The permission set, dependencies, settings schema and
webapp shown from the catalog snapshot are then exactly what gets installed,
and the permission fingerprint a host stores (SEC-9) does not depend on which
package was picked.

### Catalog shape

A release keeps every field it had and gains `downloads` (PKG-14):

```jsonc
{
  "version": "1.0.0",
  "released_at": "2026-09-22T00:00:00Z",
  "api_version": 1,
  "min_nginx_ui_version": "2.7.0",
  // Summary: keys of downloads plus the platforms the portable package serves.
  "platforms": ["linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64", "windows-arm64"],
  // New: one package per platform, or "any" for a platform independent one.
  "downloads": {
    "linux-amd64": {
      "url": "https://.../com.nginxui.dns01-1.0.0-linux-amd64.tar.gz",
      "sha256": "<lowercase hex sha256 of that file>",
      // Optional, defaults to url + ".minisig".
      "signature_url": "https://.../com.nginxui.dns01-1.0.0-linux-amd64.tar.gz.minisig"
    }
  },
  // Portable package, optional once downloads is present.
  "download_url": "https://.../com.example.plugin-1.0.0.tar.gz",
  "sha256": "<sha256 of the portable package>",
  "signature_url": "https://.../com.example.plugin-1.0.0.tar.gz.minisig",
  "signed_by": "official",
  "release_notes_url": "https://...",
  "yanked": false,
  // Snapshot of the release manifest; server.executables lists every platform.
  "manifest": { "...": "plugin.json with the full executables map" }
}
```

`schema/catalog.schema.json` is the machine-readable form of the whole
document. Field names stay snake_case, like the rest of the catalog.

### Selection

For a host on platform `P` (PKG-15):

```text
select(release, P):
    if downloads[P].url != ""          -> downloads[P]            (key P)
    if downloads["any"].url != ""      -> downloads["any"]        (key any)
    if download_url != "" and
       (platforms is empty or P in platforms or "any" in platforms)
                                        -> portable package        (no key)
    otherwise                           -> not installable on P
```

The empty `platforms` rule keeps a schema 1 catalog without `downloads`
working exactly as before. A release is installable on `P` when its
`api_version` matches, the host is at least `min_nginx_ui_version`, and
`select` returns a package.

The reference host uses the selection for:

* `installable_release` of every catalog entry, and so for the marketplace
  cards, the install dialog and the "newest version" of an update;
* the update check and the daily automatic update;
* dependency resolution while installing (`requires`);
* the automatic install of the DNS-01 plugin and the repair of official
  plugins a core upgrade left incompatible;
* `POST /api/plugins/marketplace/install`, with or without an explicit
  version (an explicit version without a build for the host fails with
  `55110`, "plugin release has no build for this platform");
* `nginx-ui plugin fetch <id> [--version v] [--platform <goos>-<goarch>[,...]|all]`,
  which defaults to the host platform, saves the file under the name of the
  form it downloaded and resolves `all` to one platform per distinct package
  of one release;
* the install progress events, which now carry `platform`: the `downloads`
  key being installed, empty for the portable package.

### Verification

Everything that applied to the portable package applies to the selected one
(PKG-16): its own `sha256`, its own signature (at `signature_url`, or
`url + ".minisig"`) under the unchanged signature policy, and after unpacking
the manifest `id` and `version` must match the entry and the release. The
reference host additionally refuses to install any package, from any source,
that has no executable for its platform and no `server.command`, with `55006`
("plugin has no executable for this platform"), instead of installing a
plugin that could never start.

### Offline packages

An operator may drop packages of every platform into `{plugins}/packages/`.
The reference host installs only what runs on it: a per-platform file of
another platform is skipped by its name, without unpacking it, and a portable
file without a build for the host is skipped after reading its manifest. Both
stay where they are, because cluster sync may push them to other nodes.

`nginx-ui plugin inspect` and the upload dialog report the platforms a
package ships (`platforms`, `"any"` for a package that runs everywhere), the
host platform (`host_platform`) and whether the package runs on it
(`platform_supported`). The upload dialog refuses to install a package for
another platform. `GET /api/plugins/spec` reports the node platform as
`platform`, and the marketplace list and detail responses report the
platform `installable_release` was resolved for as `host_platform`, which the
versions table uses to highlight the build this node would install.

### Cluster sync

A controller installed the package of its own platform, which does not fit a
node on another platform (PKG-18). The reference host resolves the node
platform from the node's plugin spec, falling back to the platform the node
monitor reports, and installs as follows:

1. The node installs from its own marketplace, which selects its own package
   (PKG-15). This stays the first choice.
2. Otherwise the controller pushes a package. For a node of an unknown
   platform, or one the controller package covers (a portable package, or
   the same platform), it pushes its own package exactly as before.
3. For another platform it looks for a package of the same id and version
   that runs there, in either naming form, in `{plugins}/packages/`,
   `{plugins}/packages/installed/` and `{plugins}/packages/cache/`. Packages
   found in the first two directories are checked against their `.minisig`
   like any offline package.
4. Otherwise it downloads the catalog package `select(release, node
   platform)` returns for that version into `{plugins}/packages/cache/`,
   verifies digest, signature, id, version and platform, keeps the signature
   next to it, and removes cached packages of other versions of the plugin.
   The cache sits under the package directory, whose scanner never looks
   into subdirectories, so a cached package is never installed on the
   controller itself.
5. When nothing exists the node is reported as `unsupported_platform` with a
   message naming the platform and what the catalog does ship; nothing is
   pushed.

The plugin matrix applies the same reasoning without downloading: a node
whose platform the installed package does not cover is `unsupported_platform`
only when no local package is named for that platform and the catalog release
of the installed version has no package for it. The catalog is only read when
such a node exists, and a node that already runs the installed version is
never flagged.

### Tooling

`nginx-ui plugin lint` reports PKG-12 as an error for an archive whose name
carries a platform suffix while `server.executables` does not declare exactly
that platform, and warns under PKG-1 when a parsable file name carries
another id or version than its manifest. PKG-9 is unchanged.

The DNS-01 plugin's `build.sh` builds one package per platform with a
narrowed `plugin.json` (`go run ./cmd/manifest -platform <goos>-<goarch> -out
<file>`), writes a `sha256sum` style `<archive>.sha256` next to each archive
for the catalog, and prints the resulting file list. `--host-only` builds and
packages the current platform only. It no longer writes the combined archive.

## Compatibility

* `schema_version` stays 1. `downloads` is an optional member (VER-4); a
  catalog without it means what it meant before.
* A host that predates this RFC ignores `downloads` and reads `download_url`
  and `platforms`. A catalog that must keep such a host working keeps a
  portable package, or points `download_url` at one per-platform package and
  lists only that platform in `platforms`. The plugin system has not shipped
  in a release before this RFC, so the official catalog does not need to.
* A portable package that declares platforms it does not ship was already a
  PKG-9 lint error; hosts still tolerate it at install time, but the reference
  host now only counts the platforms whose file is present when it decides
  what a package runs on.

## What a catalog must adopt

1. For every release with native executables, publish one per-platform
   package per platform and list each under `downloads` with `url` and
   `sha256` (from the `.sha256` file `build.sh` writes), and a signature at
   `url + ".minisig"` or an explicit `signature_url`.
2. Set `platforms` to the keys of `downloads` plus whatever the portable
   package serves.
3. Keep `download_url`, `sha256` and `signature_url` only when a portable
   package exists; omit them otherwise.
4. Set `manifest` to the full `plugin.json` of the release, whose
   `server.executables` names every platform.
5. Validate the document against `schema/catalog.schema.json`.

Example, the official DNS-01 plugin:

```json
{
  "schema_version": 1,
  "updated_at": "2026-09-23T00:00:00Z",
  "plugins": [
    {
      "id": "com.nginxui.dns01",
      "name": { "en": "DNS-01 Challenge" },
      "description": { "en": "Solve the ACME DNS-01 challenge with any of the DNS providers supported by lego." },
      "repository_url": "https://github.com/0xJacky/nginx-ui-plugin-dns01",
      "capabilities": ["dns01"],
      "license": "AGPL-3.0",
      "trust": "official",
      "stage": "production",
      "releases": [
        {
          "version": "1.0.0",
          "released_at": "2026-09-22T00:00:00Z",
          "api_version": 1,
          "min_nginx_ui_version": "2.7.0",
          "platforms": ["darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64", "windows-arm64"],
          "downloads": {
            "darwin-amd64": { "url": "https://github.com/0xJacky/nginx-ui-plugin-dns01/releases/download/v1.0.0/com.nginxui.dns01-1.0.0-darwin-amd64.tar.gz", "sha256": "<sha256>" },
            "darwin-arm64": { "url": "https://github.com/0xJacky/nginx-ui-plugin-dns01/releases/download/v1.0.0/com.nginxui.dns01-1.0.0-darwin-arm64.tar.gz", "sha256": "<sha256>" },
            "linux-amd64": { "url": "https://github.com/0xJacky/nginx-ui-plugin-dns01/releases/download/v1.0.0/com.nginxui.dns01-1.0.0-linux-amd64.tar.gz", "sha256": "<sha256>" },
            "linux-arm64": { "url": "https://github.com/0xJacky/nginx-ui-plugin-dns01/releases/download/v1.0.0/com.nginxui.dns01-1.0.0-linux-arm64.tar.gz", "sha256": "<sha256>" },
            "windows-amd64": { "url": "https://github.com/0xJacky/nginx-ui-plugin-dns01/releases/download/v1.0.0/com.nginxui.dns01-1.0.0-windows-amd64.tar.gz", "sha256": "<sha256>" },
            "windows-arm64": { "url": "https://github.com/0xJacky/nginx-ui-plugin-dns01/releases/download/v1.0.0/com.nginxui.dns01-1.0.0-windows-arm64.tar.gz", "sha256": "<sha256>" }
          },
          "signed_by": "official",
          "release_notes_url": "https://github.com/0xJacky/nginx-ui-plugin-dns01/releases/tag/v1.0.0",
          "manifest": {
            "id": "com.nginxui.dns01",
            "name": "DNS-01 Challenge",
            "version": "1.0.0",
            "api_version": 1,
            "min_nginx_ui_version": "2.7.0",
            "server": {
              "executables": {
                "darwin-amd64": "server/dist/dns01-darwin-amd64",
                "darwin-arm64": "server/dist/dns01-darwin-arm64",
                "linux-amd64": "server/dist/dns01-linux-amd64",
                "linux-arm64": "server/dist/dns01-linux-arm64",
                "windows-amd64": "server/dist/dns01-windows-amd64.exe",
                "windows-arm64": "server/dist/dns01-windows-arm64.exe"
              },
              "lifecycle": "on_demand",
              "idle_timeout_seconds": 300
            },
            "capabilities": ["dns01"],
            "permissions": ["network"],
            "dns01": { "providers": [{ "name": "1cloud.ru", "code": "onecloudru" }] }
          }
        }
      ]
    }
  ]
}
```

The `<sha256>` placeholders are the digests of the published files, and the
`manifest` is the committed `plugin.json` verbatim; the provider list, the
webapp block and the settings schema are shortened or left out here. A
webapp-only or interpreted plugin keeps publishing one portable package and
either keeps `download_url` with `"platforms": ["any"]` or lists it as
`"downloads": { "any": { ... } }`; both select the same package everywhere.

## Alternatives considered

* **One archive with every platform.** Over the PKG-7 budget for the
  official plugin and wasteful for every other native plugin.
* **A catalog entry per platform** (`com.nginxui.dns01.linux-amd64`). Splits
  one plugin into several ids, breaks `requires`, updates and the installed
  inventory, and collides with NAME-1's meaning of an id.
* **A URL template** (`download_url: ".../{goos}-{goarch}.tar.gz"`). Cannot
  carry a digest per file, and hides which platforms exist until a download
  fails.
* **Fetching only the needed member of a combined archive.** A gzip stream is
  not seekable, so a host would still download everything before the member
  it wants.
* **Keeping the full `server.executables` map in every package** and teaching
  the linter to accept missing files. It makes PKG-9 unenforceable and lets a
  package claim platforms it cannot run on.

## Security considerations

Every package carries its own digest and signature, so a mirror cannot swap
the package of one platform for another without failing verification. The id
and version check after unpacking, and the refusal to install a package
without a build for the host, stop a catalog from mislabeling a package for
another platform. A controller that fetches packages for other platforms
verifies them exactly like an install before caching them, and re-verifies
packages it did not download itself before pushing them.

## Future work

* Deterministic archives (PKG-10) are not reached with the stock `tar` of
  every platform; a Go packer in the SDK could produce identical bytes on
  every machine.
* Catalog tooling that reads the `.sha256` files and the release manifest and
  writes the release object, so the catalog is never edited by hand.
