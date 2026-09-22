# 11. Naming

The identifier namespaces of this system are plugin ids, `dns01` provider
codes, `notify` channel codes, `probe` kind codes, `storage` backend codes
and `cert.deploy` target kind codes. All of them are
effectively global — installing two plugins that collide in one namespace
produces undefined behavior — so they are specified here rather than left to
convention alone. MCP tool names are scoped to one plugin and made global by
the host (NAME-11). Package file names and
the platform keys they share with `server.executables` and catalog
`downloads` close the document.

## Plugin ids

## NAME-1

A plugin id MUST match `^[a-z0-9]+(\.[a-z0-9-]+)+$` and MUST be at most 64
characters (MAN-2): lowercase ASCII letters, digits and hyphens, arranged as
two or more dot-separated segments, most-significant segment first — a
reversed-domain style namespace, e.g. `com.example.myplugin`,
`io.github.example.mydns`.

## NAME-2

The `com.nginxui.*` namespace is reserved for plugins maintained by the
nginx-ui project itself (e.g. `com.nginxui.dns01`). A third party plugin
author MUST NOT publish a plugin id starting with `com.nginxui.`. This is a
naming policy rather than something the reference host's manifest validator
currently checks by itself (MAN-2's regex alone would accept it); a host or
plugin registry implementing distribution/publishing SHOULD enforce it at
that layer.

## NAME-3

An author without a domain name of their own SHOULD use
`io.github.<owner>.<name>`, where `<owner>` is their GitHub username or
organization (lowercased, hyphens allowed) and `<name>` is the plugin's own
name — e.g. a plugin named "mydns" published by GitHub user `example` is
`io.github.example.mydns`. This gives every GitHub user a namespace they
already control without needing a domain, mirroring the long-standing Java
package naming convention for the same problem.

## `dns01` provider codes

## NAME-4

A `dns01` provider `code` (DNS01-2) MUST match `^[a-z0-9-]{2,32}$` and lives
in a namespace shared across every installed `dns01`-capable plugin, not
just within one plugin's own manifest: the certificate options flow
(`options.provider_code`, `spec/07-webapp.md` WEB-9) selects a provider by
code alone, with the owning plugin resolved separately, so two different
plugins claiming the same code for different vendors would make that
selection ambiguous.

## NAME-5

A plugin author SHOULD choose a `code` that does not collide with a
well-known vendor's existing code, especially one already used by the
official `dns01` plugin's embedded [lego](https://github.com/go-acme/lego)
provider catalog, unless the plugin is genuinely an alternative
implementation for that same vendor and intends to replace it. A host MAY
refuse to enable a plugin that declares a `code` already registered by
another enabled plugin, and SHOULD surface the conflict to the person
installing it rather than silently letting one shadow the other.

## NAME-6

A provider `code` names the vendor, not the plugin: it MUST NOT be prefixed
with the plugin id, and MUST be stable across the plugin's own releases —
renaming a shipped `code` breaks every certificate already configured to use
it, since `provider_code` is stored per certificate, not re-derived from the
manifest at issuance time.

## Capability entry codes

## NAME-10

A `notify` channel `code` (NOTIFY-2), a `probe` kind `code` (PROBE-2), a
`storage` backend `code` (STORAGE-2) and a `cert.deploy` target kind `code`
(DEPLOY-2) MUST match `^[a-z0-9-]{2,32}$`. Each of the four forms one
namespace shared across every installed plugin of its capability: channel
codes across `notify` plugins, kind codes across `probe` plugins, backend
codes across `storage` plugins and target kind codes across `cert.deploy`
plugins. A notification channel, a health check, a backup task or a deploy
target stores the code alone, and the owning plugin is resolved at the time
of the call.
The rules NAME-5 and NAME-6 give for provider codes apply to them as well:
choose a code that names the vendor or the technique, not the plugin, and
never rename a shipped code. When several enabled plugins declare one code,
a host MUST pick one of them deterministically; the reference host picks the
lowest plugin id. A host keeps these codes apart from the names of its
built-in channels, checks and storage (NOTIFY-9, PROBE-7, STORAGE-12,
DEPLOY-10).

## MCP tool names

## NAME-11

An `mcp` tool `name` (MCP-2) is scoped to one plugin. The host publishes it
as the plugin id with every `.` replaced by `_`, followed by `__` and the
tool name: tool `purge_cache` of plugin `io.github.example.cdn` is published
as `io_github_example_cdn__purge_cache`. The published name is stable across
releases as long as the plugin id and the tool name are, and it cannot
collide: a plugin id contains no `_` (NAME-1), so the first `__` of a
published name ends the prefix and the prefix maps back to exactly one
plugin id. It stays within the 128 characters the Model Context Protocol
allows for a tool name (at most 64 + 2 + 48). Some model APIs accept only 64
characters, so a plugin author SHOULD keep the published name within 64, and
SHOULD NOT rename a shipped tool, since MCP clients and prompts refer to it
by name.

## Settings keys and slot names

## NAME-7

A `settings_schema.settings[].key` (MAN-27) is scoped to one plugin's own
settings map and needs no cross-plugin uniqueness; a plugin author MAY use
short, unprefixed keys freely (`timeout`, not `myplugin_timeout`).

## NAME-8

A custom slot name (one not in WEB-9's table) that a plugin both defines and
consumes itself SHOULD be prefixed with that plugin's id or a short unique
tag, e.g. `com.example.myplugin:extra-panel`, to avoid an accidental
collision with a slot name a future spec revision or another plugin
introduces.

## Package file names and platform keys

## NAME-9

A platform key is `<goos>-<goarch>`, where `<goos>` and `<goarch>` are the
`GOOS` and `GOARCH` values of the Go toolchain, lowercase, exactly as
`go tool dist list` prints them joined by a hyphen instead of a slash: for
example `linux-amd64`, `linux-arm64`, `darwin-arm64`, `windows-amd64`. The
same keys are used by `server.executables` (MAN-14), by a catalog release's
`platforms` and `downloads` (PKG-14), and by per-platform package file names
(PKG-1). The key `any` is reserved for "every platform": it MAY appear in
`platforms` and as a `downloads` key, and MUST NOT be used as a file name
suffix, a portable package already covers it.

A package file is named `<id>-<version>.tar.gz` when portable and
`<id>-<version>-<goos>-<goarch>.tar.gz` when built for one platform, e.g.
`com.nginxui.dns01-1.0.0.tar.gz` and
`com.nginxui.dns01-1.0.0-linux-arm64.tar.gz`. A tool reading a file name
treats it as per-platform only when its last two hyphen-separated tokens are
a known `GOOS` and a known `GOARCH`, and splits the rest at the first hyphen
that leaves a valid plugin id (NAME-1) on the left and a semantic version on
the right. A plugin author SHOULD NOT publish a version whose prerelease part
ends in such a pair (`1.0.0-linux-amd64`), since its portable package name
would read as a per-platform one.
