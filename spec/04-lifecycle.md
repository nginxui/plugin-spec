# 04. Lifecycle

This document describes the sequence of calls that take a plugin process from
"just spawned" to "gone", and the liveness contract in between. All six
methods here are host → plugin (the host calls, the plugin replies or is
notified).

| Method | Kind | Meaning |
| --- | --- | --- |
| `plugin.initialize` | request | Handshake: exchange versions and capabilities. |
| `plugin.initialized` | notification | The handshake succeeded; the plugin may now call `host.*`. |
| `plugin.configure` | request | New settings map. |
| `plugin.ping` | request | Liveness probe. |
| `plugin.shutdown` | request | Finish in-flight work, prepare to exit. |
| `plugin.exit` | notification | Exit now. |

## Handshake

## LIFE-1

Immediately after spawning the process and before sending any other method,
the host MUST send `plugin.initialize`:

```json
{
  "jsonrpc": "2.0", "id": 1, "method": "plugin.initialize",
  "params": {
    "host": { "version": "2.7.0", "os": "linux", "arch": "amd64", "locale": "en" },
    "settings": {},
    "permissions": ["network"]
  }
}
```

`settings` is the plugin's persisted settings map (`{}` on first install).
`permissions` is the set the host has actually granted, which MAY be a
subset of what the manifest requests if the person installing the plugin has
not yet approved an upgrade's new permissions (SEC-9).

## LIFE-2

The plugin MUST reply with `InitializeResult`:

```json
{
  "jsonrpc": "2.0", "id": 1,
  "result": { "api_version": 1, "capabilities": ["dns01"] }
}
```

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `api_version` | integer | yes | The wire protocol version this process implements. |
| `capabilities` | string[] | yes | Capability names this process actually implements at runtime. |
| `transports` | string[] | no | Transports the plugin serves, e.g. `["stdio", "grpc"]`. stdio is always served, listed or not; empty/absent means stdio only. Listing `grpc` opts in to the gRPC transport (`spec/03-wire-protocol.md` WIRE-11). |
| `http_port` | integer | no | Reported by a plugin serving the `http` capability on a loopback port instead of a Unix socket (Windows). |
| `rpc_port` | integer | no | Reported by a plugin serving gRPC on a loopback port instead of a Unix socket (Windows). |
| `rpc_token` | string | no | Bearer token the host sends as `authorization: Bearer <rpc_token>` on every call to `rpc_port`. |
| `rpc_socket` | string | no | Absolute path of the Unix socket the plugin serves gRPC on. Absent means `<NGINX_UI_PLUGIN_DATA_DIR>/rpc.sock`; a plugin whose default path exceeds the platform's socket path limit reports the path it used instead (WIRE-11). |

## LIFE-3

`result.api_version` MUST exactly equal the `api_version` the host itself
implements. A host MUST treat any other value — higher or lower — as a
handshake failure and MUST NOT proceed to `plugin.initialized`. There is no
partial-compatibility negotiation in spec 1.x: see `spec/10-versioning.md`.

## LIFE-4

`result.capabilities`, taken as a set, MUST be identical to the
`capabilities` array the plugin's own manifest declares. A host MUST treat a
mismatch (extra or missing entries either way) as a handshake failure. A
plugin's manifest is the advertisement; `InitializeResult` is the plugin
confirming, from inside the running process, that the advertisement is true.

## LIFE-5

A host MUST bound how long it waits for the `plugin.initialize` reply. The
reference host uses **10 seconds**; a host implementation MAY use a
different bound but SHOULD document it, since a plugin author has no other
way to know how much startup work (loading a large embedded catalog, warming
a cache) is safe to do before replying.

## LIFE-6

Once the handshake succeeds (LIFE-3 and LIFE-4 both hold), the host MUST send
the `plugin.initialized` notification:

```json
{ "jsonrpc": "2.0", "method": "plugin.initialized" }
```

A plugin MUST NOT call any `host.*` method (`spec/06-host-api.md`) before it
has received this notification, and a host MAY reject an early `host.*` call
with `-32600` (invalid request) if one arrives regardless.

## Configuration

## LIFE-7

The settings snapshot already given in `plugin.initialize.params.settings`
is not repeated automatically as a `plugin.configure` call — a host MAY send
one right after `plugin.initialized` anyway for symmetry, but in either case
MUST send `plugin.configure` every time a person saves new settings for the
plugin while it is running:

```json
{ "jsonrpc": "2.0", "id": 2, "method": "plugin.configure", "params": { "settings": { "default_propagation_timeout_seconds": 90 } } }
```

The plugin MUST reply with an empty result (`{}`) once the new settings have
taken effect for subsequent calls.

## Liveness

## LIFE-8

While a plugin process is running, the host MUST periodically send
`plugin.ping` and MUST require a timely reply:

```json
{ "jsonrpc": "2.0", "id": 3, "method": "plugin.ping" }
```

```json
{ "jsonrpc": "2.0", "id": 3, "result": {} }
```

A plugin MUST reply to `plugin.ping` even while busy handling other requests
— a host SHOULD run ping handling independently of a plugin's own request
queue, but a plugin that cannot guarantee that MUST at least prioritize
`plugin.ping` over slow capability calls.

## LIFE-9

The reference host pings every **15 seconds** with a **5 second** per-attempt
timeout, and kills the process after **3 consecutive** missed or timed-out
pings. A host implementation MAY use different numbers but SHOULD keep the
same order of magnitude: a plugin author should be able to assume "tens of
seconds of unresponsiveness, not milliseconds" before being killed for it.

## Shutdown

## LIFE-10

To stop a plugin deliberately (disable, uninstall, upgrade, an `on_demand`
plugin going idle, or host shutdown), the host MUST:

1. Send `plugin.shutdown` as a request and wait for its reply, up to a bounded
   timeout (5 seconds in the reference host). The plugin SHOULD use this step
   to finish in-flight capability calls (`dns01.*`, `http.handle`,
   `notify.send`, `probe.check`, `mcp.call`), on stdio and on gRPC
   alike (WIRE-11), and stop accepting new ones, then reply with `{}`.
2. Send `plugin.exit` as a notification, regardless of whether step 1's
   reply arrived in time.
3. Wait for the process to exit on its own, up to a second bounded timeout
   (5 seconds in the reference host).
4. If the process has not exited by the end of step 3, forcibly terminate it
   (`SIGKILL` or the platform equivalent).

## LIFE-11

On receiving `plugin.exit`, a plugin MUST exit as soon as it reasonably can
and MUST NOT wait for further input on stdin — no reply is expected or
possible, since it is a notification. A plugin serving gRPC closes its
listener on the way out. A plugin SHOULD exit with status code
`0` when it received `plugin.exit` after already replying to
`plugin.shutdown`; any other exit code, or an exit not preceded by these
messages, is treated by the host as a crash (LIFE-12).

## Crash and restart

## LIFE-12

A `resident` plugin process (`server.lifecycle` absent or `"resident"`) that
exits without having been asked to (no `plugin.shutdown`/`plugin.exit` were
sent) is a crash. A host SHOULD restart a crashed resident plugin
automatically with a backoff delay, and MUST stop restarting it and surface
an error state after too many crashes in too short a window — the reference
host restarts after 1s, 2s, 4s, 8s, then 16s delays, and gives up (enters an
error state requiring manual intervention) after 3 crashes within 5 minutes.

## LIFE-13

An `on_demand` plugin (`server.lifecycle: "on_demand"`) is started the first
time it is needed and MUST be stopped (via LIFE-10) after
`server.idle_timeout_seconds` of no active callers. A host MUST start it
again transparently on the next call. A crash of an `on_demand` plugin with
no caller currently waiting SHOULD NOT count against the crash limit in
LIFE-12 the same way a resident plugin's crash does, since nothing was
actively depending on it at that moment.

## Environment

## LIFE-14

The host MUST set these environment variables for the plugin process and
MUST NOT rely on the plugin reading them from anywhere else:

| Variable | Meaning |
| --- | --- |
| `NGINX_UI_PLUGIN_ID` | The plugin id from the manifest. |
| `NGINX_UI_PLUGIN_API_VERSION` | The `api_version` the host implements (an integer, as a string). |
| `NGINX_UI_PLUGIN_DATA_DIR` | An absolute path to a directory this plugin, and no other, may write to. |
| `NGINX_UI_VERSION` | The host application's own version string. |

## LIFE-15

The host MUST NOT forward its own ACME-client-only environment variables
(for example `LEGO_DISABLE_CNAME_SUPPORT`) into the plugin process
environment: those variables belong to the host's built-in HTTP-01 path, and
a `dns01` plugin has its own, explicit way to receive the equivalent setting
per certificate (`spec/05-capabilities-dns01.md`).
