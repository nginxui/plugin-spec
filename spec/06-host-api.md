# 06. Host API

Once a plugin has received `plugin.initialized` (LIFE-6), it may call
`host.*` methods: plugin → host requests. This document also covers the two
ways the host calls back into a plugin on its own: event delivery through the
`events.on` notification (HOST-14) and fired cron entries, which call the
entry's own method (HOST-10).

| Method | Permission required | Meaning |
| --- | --- | --- |
| `host.log` | none | Structured log line, shown alongside the plugin's stderr output. |
| `host.kv.get` | `kv` | Read one key from this plugin's own key-value store. |
| `host.kv.set` | `kv` | Write one key. |
| `host.kv.delete` | `kv` | Delete one key. |
| `host.kv.list` | `kv` | List keys by prefix. |
| `host.settings.get` | none | Read this plugin's own current settings map. |
| `host.i18n.locale` | none | Read the host's current locale. |
| `host.credentials.get` | `credentials.read:<kind>` | Read a stored credential by kind and id. |
| `host.cron.register` | `cron` | Register a cron entry at runtime. |
| `host.cron.unregister` | `cron` | Remove a previously registered cron entry. |
| `host.notify` | `notify` | Push a notification into the host's UI. |
| `host.metrics.snapshot` | `metrics.read` | Read the host's current metrics snapshot. |
| `host.logs.list` | `log.files` | List the nginx log files the plugin may read. |
| `host.activity.set` | none | Show or clear an entry of the plugin's background work in the host's processing indicator. |

## HOST-1

A host MUST enforce the "Permission required" column above by checking the
permission set it granted this plugin (`plugin.initialize.params.permissions`,
LIFE-1), not the manifest's requested set — a plugin whose upgrade added a
new permission the person has not yet approved MUST still be denied that
permission at runtime (SEC-9), even though its manifest asks for it.

## HOST-2

A host MUST reply `-32001` (Permission denied) to a `host.*` call the
caller's granted permissions do not cover, and MUST NOT partially perform
the call first.

## `host.log`

## HOST-3

```json
{ "jsonrpc": "2.0", "id": 20, "method": "host.log", "params": { "level": "info", "message": "reconfigured", "fields": { "settings_count": 2 } } }
```

`level` is one of `debug`, `info`, `warn`, `error`. Requires no permission:
logging is always available so a plugin can report its own problems even
before any permission is granted. The host MUST reply `{}`.

## `host.kv.*`

## HOST-4

```json
{ "jsonrpc": "2.0", "id": 21, "method": "host.kv.set", "params": { "key": "last_sync", "value": { "at": 1732000000 } } }
```

`key` MUST be non-empty. `value` is any JSON value; a host MUST reject a
value larger than **64 KiB** encoded with `-32602`. The store is scoped to
the calling plugin: two plugins MUST NOT be able to read or overwrite each
other's keys even if they happen to choose the same key string.

## HOST-5

```json
{ "jsonrpc": "2.0", "id": 22, "method": "host.kv.get", "params": { "key": "last_sync" } }
```

```json
{ "jsonrpc": "2.0", "id": 22, "result": { "value": { "at": 1732000000 }, "found": true } }
```

`found: false` with `value` absent/`null` when the key does not exist — this
is not an error.

## HOST-6

`host.kv.delete` takes the same params as `host.kv.get` and MUST reply `{}`
whether or not the key existed. `host.kv.list` takes `{ "prefix"?: string }`
and replies `{ "keys": string[] }` (an empty array, never `null`, when
nothing matches).

## `host.settings.get`

## HOST-7

```json
{ "jsonrpc": "2.0", "id": 23, "method": "host.settings.get" }
```

```json
{ "jsonrpc": "2.0", "id": 23, "result": { "settings": { "default_propagation_timeout_seconds": 120 } } }
```

Requires no permission. Returns the same map the most recent
`plugin.initialize`/`plugin.configure` delivered; it exists so a plugin does
not have to cache that map itself across calls.

## `host.i18n.locale`

## HOST-8

```json
{ "jsonrpc": "2.0", "id": 24, "method": "host.i18n.locale" }
```

```json
{ "jsonrpc": "2.0", "id": 24, "result": { "locale": "zh_CN" } }
```

Requires no permission. A plugin MAY use this to localize `host.notify`
content or its own log messages.

## `host.credentials.get`

## HOST-9

```json
{ "jsonrpc": "2.0", "id": 25, "method": "host.credentials.get", "params": { "kind": "dns", "id": "7" } }
```

```json
{ "jsonrpc": "2.0", "id": 25, "result": { "id": "7", "name": "Cloudflare - example.com", "provider_code": "cloudflare", "config": { "CF_DNS_API_TOKEN": "..." } } }
```

`kind` and `id` MUST both be non-empty (else `-32602`). The permission
checked is `credentials.read:<kind>`, e.g. `credentials.read:dns` — a
manifest requesting `credentials.read:dns` MUST NOT thereby gain access to a
credential of a different kind. This method exists for a plugin that needs
to look up a credential outside the normal `dns01.present` flow (where the
credential's values already arrive in `config`); most `dns01` plugins never
call it.

## `host.cron.*`

## HOST-10

```json
{ "jsonrpc": "2.0", "id": 26, "method": "host.cron.register", "params": { "id": "refresh-catalog", "schedule": "@every 24h", "method": "catalog.refresh" } }
```

`id`, `schedule` and `method` MUST all be non-empty (else `-32602`).
`schedule` is a five field cron expression or `"@every <duration>"`.
`method` names a method the plugin serves. When the schedule fires, the host
calls that method directly, as an ordinary JSON-RPC **request** (not a
notification and not `events.on`):

```json
{ "jsonrpc": "2.0", "id": 31, "method": "catalog.refresh", "params": { "type": "refresh-catalog", "ts": 1732000000 } }
```

`params` has the shape of the `events.on` params (`EventsOnRequest`, HOST-14):
`type` is the cron entry's `id`, `ts` the Unix seconds when the host fired
it, and `data` is absent. The plugin MUST answer the request; the host
ignores the result (`{}` is conventional) and logs an error reply without
retrying. The host bounds the call (10 minutes in the reference host),
starts an `on_demand` plugin for it (LIFE-13), and does not start an entry
again while its previous invocation is still running. `method` SHOULD be a
name of the plugin's own, such as `catalog.refresh`, rather than an rpc of
the contract; such a call travels on stdio (WIRE-11).

A host MUST reject registering the same `id` twice for the same plugin by
either replacing the previous entry or erroring consistently; it MUST NOT
silently run both.

## HOST-11

`host.cron.unregister` takes `{ "id": string }` and MUST reply `{}` whether
or not that id was registered.

## `host.notify`

## HOST-12

```json
{ "jsonrpc": "2.0", "id": 27, "method": "host.notify", "params": { "level": "warning", "title": "Propagation slow", "content": "example.com is taking longer than usual to propagate.", "details": { "domain": "example.com", "elapsed_seconds": 90 } } }
```

`level` is one of `info`, `success`, `warning`, `error`. `title` and
`content` are shown to the person; `details`, when present, is structured
context the host MAY show on demand rather than inline. The host MUST reply
`{}` once the notification is queued (delivery/read state is host UI
concern, not part of this call).

## `host.metrics.snapshot`

## HOST-13

```json
{ "jsonrpc": "2.0", "id": 28, "method": "host.metrics.snapshot" }
```

```json
{ "jsonrpc": "2.0", "id": 28, "result": { "snapshot": { "cpu_percent": 3.2, "memory_bytes": 104857600 } } }
```

`snapshot`'s shape is host-defined and opaque to this spec — it mirrors
whatever the host's own metrics subsystem produces at the time of the call.
A plugin MUST treat it as read-only, best-effort telemetry, not a stable
schema to build alerting logic against across host versions.

## `host.logs.list`

## HOST-17

```json
{ "jsonrpc": "2.0", "id": 29, "method": "host.logs.list" }
```

```json
{ "jsonrpc": "2.0", "id": 29, "result": { "logs": [
  { "path": "/var/log/nginx/access.log", "type": "access", "source": "default" },
  { "path": "/var/log/nginx/example.com.error.log", "type": "error", "source": "config", "config_file": "/etc/nginx/sites-enabled/example.com.conf" }
] } }
```

Requires the `log.files` permission (SEC-30). There are no params. `logs` is
an array, never `null`. Each entry has:

| Field | Type | Meaning |
| --- | --- | --- |
| `path` | string | Absolute path of the live log file. |
| `type` | string | `access` or `error`. |
| `source` | string | `config` when an nginx configuration file names the path, `default` when it is one of nginx's built-in default log paths. |
| `config_file` | string | The configuration file that names the path. Absent when `source` is `default`. |

A host MUST list only paths it allows reading itself, the same paths it would
show a person in its own log viewer, and MUST NOT list a path twice. Rotated
files (`access.log.1`, `access.log.2.gz`, `access.log-20260101` and the like)
are not listed: a plugin that wants history looks for them next to a listed
path and reads them itself. A plugin MUST read only listed paths and the
rotated files of a listed path (SEC-30). The list is a snapshot; a plugin
calls the method again when it receives `log.paths_changed` (HOST-18).

## Event delivery: `events.on`

## HOST-14

The host delivers subscribed events to a plugin with this notification, host
→ plugin, never answered. Fired cron entries do not use it; they call the
entry's own method (HOST-10).

```json
{ "jsonrpc": "2.0", "method": "events.on", "params": { "type": "cert.renewed", "data": { "domain": "example.com" }, "ts": 1732000000 } }
```

| Field | Type | Meaning |
| --- | --- | --- |
| `type` | string | An event type from the table below. |
| `data` | any | Event payload. |
| `ts` | integer | Unix seconds when the host sent the notification. |

## HOST-15

A host MUST deliver `events.on` only for event types listed in the plugin's
manifest `events` array (MAN-1 table), and MUST call a cron `method` only for
an entry the plugin itself registered (via `cron` in the manifest, MAN-26, or
via `host.cron.register`, HOST-10). A plugin MUST tolerate
receiving a `type` it does not recognize (e.g. a spec revision added one) by
ignoring it rather than treating it as an error — `events.on` is a
notification and there is no error channel back to the host for it anyway.

## HOST-16

Defined event types:

| Type | Meaning |
| --- | --- |
| `cert.issued` | A certificate finished issuing. |
| `cert.renewed` | A certificate was renewed. |
| `cert.expiring` | A certificate is approaching expiry. |
| `site.saved` | A site configuration was saved. |
| `site.enabled` | A site was enabled. |
| `site.disabled` | A site was disabled. |
| `nginx.reloaded` | nginx finished reloading. |
| `nginx.reload_failed` | An nginx reload failed. |
| `node.status_changed` | A cluster node's status changed. |
| `node.joined` | A node joined the cluster. |
| `backup.completed` | A backup finished. |
| `auth.login_failed` | A login attempt failed. |
| `plugin.changed` | A plugin was installed, upgraded, enabled or disabled. |
| `log.paths_changed` | The set of nginx log files `host.logs.list` returns changed, for example after a configuration scan found or lost a log directive. Delivered only to a plugin holding `log.files`. See HOST-18. |

A host MUST NOT invent a new event type without a change to this spec; a
plugin author needs this list to be exhaustive to know what it can subscribe
to at all.

## HOST-18

`log.paths_changed` is an ordinary event (HOST-14, HOST-15) with one addition:
a host MUST send it only to a plugin whose granted permission set (SEC-4)
contains `log.files`, whatever its manifest `events` array says, and, as for
every event, only when that array lists the type. The host sends it when the
set of paths `host.logs.list` would return changed. `data` MAY be absent or an
empty object and carries nothing a plugin needs: it MUST call `host.logs.list`
again to learn the new set. A host SHOULD coalesce a burst of changes into one
notification.

## `host.activity.set`

## HOST-19

```json
{ "jsonrpc": "2.0", "id": 30, "method": "host.activity.set", "params": { "key": "indexing", "label": "Nginx Log Indexing...", "active": true } }
```

Requires no permission. It shows the plugin's background work in the host's
global processing indicator, next to the host's own tasks. `key` identifies
the entry inside this plugin, MUST be 1 to 64 characters of `[a-z0-9._-]`, and
`label` MUST be 1 to 128 characters (else `-32602`). `active: true` creates
the entry or replaces its label, `active: false` removes it and ignores
`label`. Removing an entry that does not exist is not an error. The host MUST
reply `{}`.

Entries are namespaced by the calling plugin id: a plugin only ever reads,
changes or clears its own entries, and two plugins that use the same `key` do
not affect each other. `label` is an English source string, not a translated
text. The host translates it with its own gettext, in the language of the
person looking at the indicator, and shows the source string when it has no
translation. A plugin supplies translations for its labels in the browser
bundle with `registerTranslations` (WEB-7), keyed by the same English string.

A host MUST remove every entry of a plugin when the plugin stops, crashes,
is disabled or is uninstalled, so a plugin that dies while `active` cannot
leave the indicator on. A host MAY limit the number of entries per plugin and
reply `-32602` beyond it; it MUST allow at least 8.
