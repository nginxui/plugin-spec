# 01. Manifest

Every plugin package carries a `plugin.json` at its root: the protobuf JSON
mapping of the `Manifest` message in
[`proto/nginxui/plugin/v1/manifest.proto`](../proto/nginxui/plugin/v1/manifest.proto),
parsed into `internal/plugin/protocol.Manifest` in the reference host. This document
lists every field and the validation the reference host applies
(`internal/plugin/manifest.go`, `ValidateManifest`). A conformant host MUST
apply at least the validation described here before enabling a plugin; a
conformant plugin package MUST satisfy it.

The full JSON Schema is at [`schema/plugin.schema.json`](../schema/plugin.schema.json).
It is tested against `manifest.proto`, see [`schema/README.md`](../schema/README.md).

## Top level fields

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `id` | string | yes | Globally unique plugin identifier. See `spec/11-naming.md`. |
| `name` | string | yes | Human readable display name. |
| `version` | string | yes | The plugin's own release version (semver). |
| `description` | string | no | One line summary shown in the plugin list. |
| `i18n` | map\<string, object\> | no | Translations of `name` and `description` keyed by locale code. See MAN-40. |
| `homepage_url` | string | no | Documentation or repository link. |
| `icon_path` | string | no | Relative path to an icon file inside the package. |
| `api_version` | integer | yes | The wire protocol version this plugin speaks. See `spec/10-versioning.md`. |
| `min_nginx_ui_version` | string | no | Advisory minimum host application version. |
| `server` | object | no\* | Declares a server process. See below. |
| `webapp` | object | no\* | Declares a browser bundle or zero-build pages. See `spec/07-webapp.md`. |
| `content` | object | no\* | Declares process-less contributions (templates, locale files). See `spec/17-content-plugins.md`. |
| `capabilities` | string[] | no | Capability names this plugin implements. |
| `permissions` | string[] | no | Host API permissions this plugin requests. See `spec/08-security.md`. |
| `requires` | object[] | no | Other plugin ids (with an optional version range) this plugin depends on. |
| `requires_capabilities` | string[] | no | Capability names this plugin expects some other installed plugin to provide. |
| `conflicts` | string[] | no | Plugin ids that must never be enabled at the same time as this plugin. See MAN-42. |
| `events` | string[] | no | Event type identifiers this plugin subscribes to. See `spec/06-host-api.md`. |
| `cron` | object[] | no | Host-scheduled invocations. See `spec/06-host-api.md`. |
| `network_hosts` | string[] | no | Declarative list of hosts the plugin intends to contact, for permission review. |
| `dns01` | object | no\*\* | Required when `capabilities` includes `dns01`. See `spec/05-capabilities-dns01.md`. |
| `http` | object | no\*\* | Required when `capabilities` includes `http`. |
| `settings_schema` | object \| null | no | Drives the auto-generated settings form. |
| `notify` | object | no\*\* | Required when `capabilities` includes `notify`. See `spec/12-capabilities-notify.md`. |
| `probe` | object | no\*\* | Required when `capabilities` includes `probe`. See `spec/13-capabilities-probe.md`. |
| `mcp` | object | no\*\* | Required when `capabilities` includes `mcp`. See `spec/14-capabilities-mcp.md`. |
| `storage` | object | no\*\* | Required when `capabilities` includes `storage`. See `spec/15-capabilities-storage.md`. |
| `deploy` | object | no\*\* | Required when `capabilities` includes `cert.deploy`. See `spec/16-capabilities-deploy.md`. |
| `blocklist` | object | no\*\* | Required when `capabilities` includes `security.blocklist`. See `spec/18-capabilities-blocklist.md`. |
| `discovery` | object | no\*\* | Required when `capabilities` includes `upstream.discovery`. See `spec/19-capabilities-discovery.md`. |
| `log_sink` | object | no | Tunes the `log.sink` capability. See `spec/20-capabilities-logsink.md`. |

\* At least one of `server`, `webapp` or `content` MUST be present.
\*\* See MAN-21, MAN-22 and MAN-31 through MAN-38 below.

### MAN-1

A `plugin.json` MUST be a single UTF-8 encoded JSON object matching
[`schema/plugin.schema.json`](../schema/plugin.schema.json).

### MAN-2

`id` MUST be non-empty, MUST be at most 64 characters, and MUST match
`^[a-z0-9]+(\.[a-z0-9-]+)+$` (a dotted, lowercase namespace with at least two
segments). See `spec/11-naming.md` for the namespace rules.

### MAN-3

`name` MUST be a non-empty string.

### MAN-4

`version` MUST be a valid [Semantic Versioning 2.0.0](https://semver.org/)
string:

```
^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$
```

### MAN-5

`api_version` MUST be present and non-zero. Under spec 1.x it MUST equal `1`.
A host MUST reject (not silently ignore) a manifest whose `api_version` does
not match a version it implements; see `spec/10-versioning.md`.

### MAN-6

`min_nginx_ui_version`, when present, is an advisory minimum host application
version. It is not a semver range and is not validated by the reference
host's manifest validator; a host MAY compare it against its own running
version and refuse to enable the plugin, and SHOULD surface it to the person
installing the plugin either way.

### MAN-7

A manifest MUST declare at least one of `server`, `webapp` or `content`. A
manifest with none of the three is invalid: it would install to nothing. A
manifest without `server` has no process and declares no capability
(CONTENT-1).

### MAN-8

`icon_path`, when present, MUST be a safe relative path as defined in
PKG-3.

## `i18n`

```json
"i18n": {
  "zh_CN": { "name": "DNS-01 验证", "description": "使用 lego 支持的任意 DNS 服务商完成 ACME DNS-01 验证。" },
  "ja_JP": { "name": "DNS-01 チャレンジ" }
}
```

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `name` | string | no | `name` in this language. |
| `description` | string | no | `description` in this language. |

### MAN-40

`i18n`, when present, maps a locale code to the translation of the top
level `name` and `description` into that language. Every key MUST be a
language the host translates its own interface into, the set CONTENT-6 lists
for translation files; a host MUST reject a manifest with any other key.
`en` is allowed but redundant, since the top level fields are the English
text.

An empty or absent field means no translation. The top level fields keep
their rules (`name` is required by MAN-3, `description` is optional) and are
the fallback: a host that shows a plugin's name or description SHOULD show,
for its active interface language, the first non-empty value of the
translation for that exact locale, for its base language (the part before
`_`), for `en`, the top level field, and finally any other translation.
Messages a host writes to its logs or sends as notifications MAY keep using
the top level `name`. A catalog MAY fill the locale maps of its own entry
from this block.

## `server`

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `executables` | map\<string, string\> | no\* | `"<goos>-<goarch>"` (e.g. `"linux-amd64"`) to a path relative to the package root. |
| `command` | string[] | no\* | Fallback argv for an interpreted plugin (e.g. `["python3", "server/main.py"]`). |
| `lifecycle` | string | no | `"resident"` (default) or `"on_demand"`. |
| `idle_timeout_seconds` | integer | no | For `on_demand`: seconds of no acquisition before the host stops the process. |
| `resources` | object | no | Resource hints: `memory_mb`, `cpu_percent` and `recommended_memory_mb`, see MAN-39, LIFE-16 and LIFE-19. |

\* At least one of `executables` or `command` MUST be present when `server` is present.

### MAN-9

When `server` is present, it MUST declare at least one of `executables` or
`command`.

### MAN-10

`server.lifecycle`, when present, MUST be exactly `"resident"` or
`"on_demand"`. An absent value means `"resident"`.

### MAN-11

`server.idle_timeout_seconds` MUST NOT be negative.

### MAN-12

Every value of `server.executables` MUST be a safe relative path (PKG-3).
`server.command[0]`, when it contains a path separator, MUST also be a safe
relative path; when it does not contain a separator it is resolved as a
program name on `PATH` (e.g. `"python3"`) and PKG-3 does not apply to it.

### MAN-13

When `server.command` is present, `server.command[0]` MUST NOT be an empty
string.

### MAN-14

A host MUST prefer `server.executables[<goos>-<goarch>]` for the platform it
runs on when present, and MUST fall back to `server.command` otherwise. A
host MUST fail to start the plugin (rather than silently skipping it) when
neither is available for its platform.

### MAN-39

`server.resources`, when present, declares what the process needs at most
and the memory the machine should have:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `memory_mb` | integer | no | Memory in MiB. `0` or absent means no hint. |
| `cpu_percent` | integer | no | CPU time in percent of one core, `100` being one core and `250` two and a half. `0` or absent means no hint. |
| `recommended_memory_mb` | integer | no | Memory in MiB that the machine, or the container the host runs in, should have for the plugin to work well. `0` or absent means no hint. |

All three MUST NOT be negative. `memory_mb` and `cpu_percent` are hints: a
host that confines plugin processes applies the smaller of a hint and its own
limit, and a host that does not ignores them (LIFE-16). A plugin author SHOULD leave headroom, since
a process that exceeds `memory_mb` under confinement is killed.

`recommended_memory_mb` is advice for the people who choose plugins, not a
limit on the process. It is the total memory of the machine, or the memory
limit of the container the host runs in, counting nginx-ui and the plugin
together. A host never refuses to install or run a plugin because of it, and
shows it as LIFE-19 describes.

## `webapp`

See `spec/07-webapp.md` for the full contract. The manifest-level shape is:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `bundle_path` | string | no | Relative path to the IIFE bundle. |
| `style_path` | string | no | Relative path to the bundle's stylesheet. |
| `shared` | map\<string, string\> | no | Shared runtime library name to the semver range the bundle was built against. |
| `pages` | object[] | no | Zero-build iframe pages. |
| `chunks` | map\<string, string\> | no | Extra IIFE files the bundle loads on demand: chunk name to a relative path. See MAN-41 and WEB-13. |

### MAN-15

`webapp.bundle_path` and `webapp.style_path`, when present, MUST be safe
relative paths (PKG-3).

### MAN-16

Each entry of `webapp.pages` MUST declare a non-empty `path` and a `file`
that is a safe relative path (PKG-3). `title` MUST be present and SHOULD
contain at least an `"en"` key.

### MAN-17

`webapp.shared`, when present, MUST map a shared runtime library name (e.g.
`"vue"`, `"antdv-next"`) to a string the host's semver range matcher accepts
(WEB-6).

### MAN-41

`webapp.chunks`, when present, declares the on-demand chunks of the bundle
(WEB-13). It MUST NOT be present unless `webapp.bundle_path` is, since the
entry is what loads a chunk. Every key MUST match `^[a-z0-9][a-z0-9_-]{0,31}$`.
Every value MUST be a safe relative path (PKG-3) that ends in `.js`, MUST NOT
equal `webapp.bundle_path`, and MUST NOT appear under two names. A host MUST
reject a manifest that breaks any of these. A package that lacks a file a
value names is invalid, which the reference linter reports
(`spec/09-conformance.md`).

## `content`

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `templates` | string | no | Relative path to a directory of nginx config snippet templates. |
| `locales` | string | no | Relative path to a directory of translation files. |

### MAN-18

`content.templates` and `content.locales`, when present, MUST be safe
relative paths (PKG-3). The layout and format of the files they point at are
specified in `spec/17-content-plugins.md` (CONTENT-2 through CONTENT-7).

## `capabilities` and their blocks

### MAN-19

Each entry of `capabilities` MUST be one of the capability names this spec
defines (currently `"dns01"`, `"http"`, `"notify"`, `"probe"`, `"mcp"`,
`"storage"`, `"cert.deploy"`, `"security.blocklist"`,
`"upstream.discovery"`, `"log.sink"`; see `spec/05-capabilities-dns01.md`,
`spec/06-host-api.md`, `spec/12-capabilities-notify.md`,
`spec/13-capabilities-probe.md`, `spec/14-capabilities-mcp.md`,
`spec/15-capabilities-storage.md`, `spec/16-capabilities-deploy.md`,
`spec/18-capabilities-blocklist.md`,
`spec/19-capabilities-discovery.md` and
`spec/20-capabilities-logsink.md`). An unknown capability name MUST cause
the host to reject the manifest.

### MAN-20

`capabilities` MUST NOT contain the same name twice.

### MAN-21

When `capabilities` includes `"dns01"`, the manifest MUST include a `dns01`
block with at least one entry in `dns01.providers`. See `spec/05-capabilities-dns01.md`.

### MAN-22

When `capabilities` includes `"http"`, the manifest MUST include an `http`
block whose `listen` field is exactly `"unix"` or `"rpc"`. `"unix"` means the
plugin serves HTTP on a listener of its own, see LIFE-18 for where it listens
and how the host finds it, and `"rpc"` that the host sends every request as one
`http.handle` call.

### MAN-31

When `capabilities` includes `"notify"`, the manifest MUST include a `notify`
block with at least one entry in `notify.channels`. See
`spec/12-capabilities-notify.md`.

### MAN-32

When `capabilities` includes `"probe"`, the manifest MUST include a `probe`
block with at least one entry in `probe.kinds`. See
`spec/13-capabilities-probe.md`.

### MAN-33

When `capabilities` includes `"mcp"`, the manifest MUST include an `mcp`
block with at least one entry in `mcp.tools`, and `permissions` MUST include
`"mcp"`. See `spec/14-capabilities-mcp.md` and SEC-13.

### MAN-34

When `capabilities` includes `"storage"`, the manifest MUST include a
`storage` block with at least one entry in `storage.backends`. See
`spec/15-capabilities-storage.md`.

### MAN-35

When `capabilities` includes `"cert.deploy"`, the manifest MUST include a
`deploy` block with at least one entry in `deploy.targets`, and
`permissions` MUST include `"cert.deploy"`. See
`spec/16-capabilities-deploy.md` and SEC-14.

### MAN-36

When `capabilities` includes `"security.blocklist"`, the manifest MUST
include a `blocklist` block with at least one entry in `blocklist.sources`,
and `permissions` MUST include `"network"`. See
`spec/18-capabilities-blocklist.md`.

### MAN-37

When `capabilities` includes `"upstream.discovery"`, the manifest MUST
include a `discovery` block with at least one entry in
`discovery.providers`, and `permissions` MUST include `"network"`. See
`spec/19-capabilities-discovery.md`.

### MAN-38

When `capabilities` includes `"log.sink"`, `permissions` MUST include
`"log.read"`. The `log_sink` block is optional; when present, `batch_size`
MUST be between `0` and `4096`, `flush_interval_ms` MUST be `0` or at least
`50`, and every entry of `formats` MUST be a format name of LOGSINK-3 and
MUST NOT appear twice. See `spec/20-capabilities-logsink.md` and SEC-16.

## `permissions`

### MAN-23

Each entry of `permissions` MUST be one of the fixed permission names
(`kv`, `network`, `cron`, `notify`, `metrics.read`, `core_api`, `mcp`,
`cert.deploy`, `log.read`, `log.files`) or MUST match
`^credentials\.read:.+$` with a non-empty kind after the colon. See
`spec/08-security.md` for what each permission gates.

## `requires` and `requires_capabilities`

### MAN-24

Each entry of `requires` is an object `{ "id": string, "version"?: string }`.
`id` MUST be a valid plugin id (MAN-2). `version`, when present, is a semver
range the host SHOULD check against the required plugin's installed version.
Dependency resolution (missing dependency, dependency cycle) is host
behavior, not part of the wire protocol; a host MUST refuse to enable a
plugin whose `requires` cannot be satisfied.

### MAN-25

`requires_capabilities` lists capability names (not plugin ids) this plugin
expects some other enabled plugin to provide. A host SHOULD warn, and MAY
refuse to enable the plugin, when no enabled plugin provides a listed
capability.

## `conflicts`

### MAN-42

`conflicts` lists plugin ids that MUST NOT be enabled at the same time as
this plugin. Every entry MUST be a valid plugin id (MAN-2), MUST NOT be the
id of the plugin itself, MUST NOT appear twice, and MUST NOT also be the id of
an entry of `requires`. A host MUST reject a manifest that breaks one of
these rules.

The relation is symmetric: two plugins conflict when either of them lists the
other, so a plugin can declare a conflict with a plugin that does not know
about it. What a host does with a conflict is described in LIFE-20.

## `events` and `cron`

See `spec/06-host-api.md` for the event type identifiers and the cron
delivery mechanism.

### MAN-26

Each entry of `cron` is an object `{ "id": string, "schedule": string, "method": string }`,
all three non-empty. `schedule` is a five field cron expression or
`"@every <duration>"`. A manifest-declared cron entry is registered by the
host at plugin startup with the same semantics as a `host.cron.register`
call (HOST-10); it does not additionally require the `cron` permission,
because the host — not the plugin — is registering it.

## Settings schema

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `header` | string | no | Text shown above the generated form. |
| `footer` | string | no | Text shown below the generated form. |
| `settings` | object[] | yes | The fields of the form. |

Each entry of `settings`:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `key` | string | yes | Settings map key. |
| `type` | string | yes | `text`, `bool`, `number`, `select`, `secret`, `textarea` or `list`. |
| `display_name` | string | yes | Field label. |
| `help_text` | string | no | Field description. |
| `default` | any | no | Default value. |
| `options` | object[] | required for `select` | `{ "value": string, "label": string }[]`. |
| `required` | boolean | no | Whether the field must be filled in. |

### MAN-27

`settings_schema.settings[].key` MUST be unique within the manifest.

### MAN-28

`settings_schema.settings[].type` MUST be one of `text`, `bool`, `number`,
`select`, `secret`, `textarea`, `list`. A `list` field holds an array of
strings; its `default`, when present, MUST be an array of strings, and the
host renders it as an editable list of single line entries.

### MAN-29

A `settings_schema.settings[]` entry whose `type` is `"select"` MUST declare
at least one entry in `options`.

### MAN-30

A `type: "secret"` field's value MUST be write-only from the host's point of
view: when the host renders the current value back to a person (e.g. in an
edit form), it MUST substitute a placeholder rather than the real value, and
MUST treat an unchanged placeholder on save as "keep the existing value" (see
SEC-8).
