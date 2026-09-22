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
| `homepage_url` | string | no | Documentation or repository link. |
| `icon_path` | string | no | Relative path to an icon file inside the package. |
| `api_version` | integer | yes | The wire protocol version this plugin speaks. See `spec/10-versioning.md`. |
| `min_nginx_ui_version` | string | no | Advisory minimum host application version. |
| `server` | object | no\* | Declares a server process. See below. |
| `webapp` | object | no\* | Declares a browser bundle or zero-build pages. See `spec/07-webapp.md`. |
| `content` | object | no\* | Declares process-less contributions (templates, locale files). |
| `capabilities` | string[] | no | Capability names this plugin implements. |
| `permissions` | string[] | no | Host API permissions this plugin requests. See `spec/08-security.md`. |
| `requires` | object[] | no | Other plugin ids (with an optional version range) this plugin depends on. |
| `requires_capabilities` | string[] | no | Capability names this plugin expects some other installed plugin to provide. |
| `events` | string[] | no | Event type identifiers this plugin subscribes to. See `spec/06-host-api.md`. |
| `cron` | object[] | no | Host-scheduled invocations. See `spec/06-host-api.md`. |
| `network_hosts` | string[] | no | Declarative list of hosts the plugin intends to contact, for permission review. |
| `dns01` | object | no\*\* | Required when `capabilities` includes `dns01`. See `spec/05-capabilities-dns01.md`. |
| `http` | object | no\*\* | Required when `capabilities` includes `http`. |
| `settings_schema` | object \| null | no | Drives the auto-generated settings form. |

\* At least one of `server`, `webapp` or `content` MUST be present.
\*\* See MAN-16/MAN-17 below.

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
manifest with none of the three is invalid: it would install to nothing.

### MAN-8

`icon_path`, when present, MUST be a safe relative path as defined in
PKG-3.

## `server`

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `executables` | map\<string, string\> | no\* | `"<goos>-<goarch>"` (e.g. `"linux-amd64"`) to a path relative to the package root. |
| `command` | string[] | no\* | Fallback argv for an interpreted plugin (e.g. `["python3", "server/main.py"]`). |
| `lifecycle` | string | no | `"resident"` (default) or `"on_demand"`. |
| `idle_timeout_seconds` | integer | no | For `on_demand`: seconds of no acquisition before the host stops the process. |

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

## `webapp`

See `spec/07-webapp.md` for the full contract. The manifest-level shape is:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `bundle_path` | string | no | Relative path to the IIFE bundle. |
| `style_path` | string | no | Relative path to the bundle's stylesheet. |
| `shared` | map\<string, string\> | no | Shared runtime library name to the semver range the bundle was built against. |
| `pages` | object[] | no | Zero-build iframe pages. |

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

## `content`

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `templates` | string | no | Relative path to a directory of nginx config snippet templates. |
| `locales` | string | no | Relative path to a directory of translation files. |

### MAN-18

`content.templates` and `content.locales`, when present, MUST be safe
relative paths (PKG-3).

## `capabilities` and their blocks

### MAN-19

Each entry of `capabilities` MUST be one of the capability names this spec
defines (currently `"dns01"`, `"http"`; see `spec/05-capabilities-dns01.md`
and `spec/06-host-api.md`). An unknown capability name MUST cause the host to
reject the manifest.

### MAN-20

`capabilities` MUST NOT contain the same name twice.

### MAN-21

When `capabilities` includes `"dns01"`, the manifest MUST include a `dns01`
block with at least one entry in `dns01.providers`. See `spec/05-capabilities-dns01.md`.

### MAN-22

When `capabilities` includes `"http"`, the manifest MUST include an `http`
block whose `listen` field is exactly `"unix"` or `"rpc"`.

## `permissions`

### MAN-23

Each entry of `permissions` MUST be one of the fixed permission names
(`kv`, `network`, `cron`, `notify`, `metrics.read`, `core_api`) or MUST match
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
| `type` | string | yes | `text`, `bool`, `number`, `select`, `secret` or `textarea`. |
| `display_name` | string | yes | Field label. |
| `help_text` | string | no | Field description. |
| `default` | any | no | Default value. |
| `options` | object[] | required for `select` | `{ "value": string, "label": string }[]`. |
| `required` | boolean | no | Whether the field must be filled in. |

### MAN-27

`settings_schema.settings[].key` MUST be unique within the manifest.

### MAN-28

`settings_schema.settings[].type` MUST be one of `text`, `bool`, `number`,
`select`, `secret`, `textarea`.

### MAN-29

A `settings_schema.settings[]` entry whose `type` is `"select"` MUST declare
at least one entry in `options`.

### MAN-30

A `type: "secret"` field's value MUST be write-only from the host's point of
view: when the host renders the current value back to a person (e.g. in an
edit form), it MUST substitute a placeholder rather than the real value, and
MUST treat an unchanged placeholder on save as "keep the existing value" (see
SEC-8).
