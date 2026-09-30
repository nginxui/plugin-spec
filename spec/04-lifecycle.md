# 04. Lifecycle

This document describes the sequence of calls that take a plugin process from
"just spawned" to "gone", and the liveness contract in between. All six
methods here are host → plugin (the host calls, the plugin replies or is
notified). A plugin without a `server` block has no process, and nothing in
this chapter applies to it (CONTENT-1).

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
| `transports` | string[] | no | Transports the plugin serves, e.g. `["stdio", "grpc"]`. stdio is always served, listed or not; empty/absent means stdio only. Listing `grpc` opts in to the gRPC transport (`spec/03-wire-protocol.md` WIRE-11). A `log.sink` plugin MUST list `grpc` (LOGSINK-4). |
| `http_port` | integer | no | Reported by a plugin serving the `http` capability on a loopback port instead of a Unix socket (Windows), see LIFE-18. |
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
   `notify.send`, `probe.check`, `mcp.call`, `storage.*`, `deploy.push`,
   `blocklist.fetch`, `discovery.resolve`) and open `log.push` streams, on
   stdio and on gRPC alike (WIRE-11, WIRE-12), and stop accepting new ones,
   then reply with `{}`. The host opens no new stream once it sent
   `plugin.shutdown`.
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

A host sets the proxy variables under the conditions of LIFE-17, and
`NGINX_UI_PLUGIN_HTTP_SECRET` under those of LIFE-18. A host that runs as a
public demo sets `NGINX_UI_DEMO` to `1`, and leaves it unset otherwise, so a
plugin can replace data a demo cannot provide with placeholders. The value
comes from the host's own configuration, never from its inherited
environment.

## LIFE-15

The host MUST NOT forward its own ACME-client-only environment variables
(for example `LEGO_DISABLE_CNAME_SUPPORT`) into the plugin process
environment: those variables belong to the host's built-in HTTP-01 path, and
a `dns01` plugin has its own, explicit way to receive the equivalent setting
per certificate (`spec/05-capabilities-dns01.md`).

## Proxy

## LIFE-17

A host that has its own outbound HTTP proxy configuration SHOULD hand it to a
plugin process that holds the `network` permission (SEC-2) through the
environment, so the plugin's HTTP client reaches the internet the way the
host does:

| Variable | Meaning |
| --- | --- |
| `HTTP_PROXY` and `http_proxy` | The proxy URL for plain HTTP requests. |
| `HTTPS_PROXY` and `https_proxy` | The proxy URL for HTTPS requests. |
| `NO_PROXY` and `no_proxy` | The hosts that bypass the proxy. It lists at least the loopback addresses (`localhost,127.0.0.1,::1`), plus whatever the host configuration exempts. |

The host sets them only while a proxy is configured, from its own proxy
configuration, and both spellings of a variable carry the same value. They
replace any proxy variable the host process itself inherited. When the host
has no proxy configuration, the plugin process inherits the environment
unchanged.

A host MUST NOT pass any of these variables, set from its configuration or
inherited from its own environment, to a plugin that does not hold `network`:
such a plugin has no business making outbound requests, and a proxy address
may embed credentials.

The variables are read by the standard HTTP clients of most languages, for
example `http.ProxyFromEnvironment` in Go. They are read when the process
starts, so a changed proxy configuration reaches a plugin on its next start.
They say nothing about the `host.*` calls, which never leave the machine.

## HTTP listener

## LIFE-18

A plugin that declares the `http` capability with `http.listen` `"unix"`
(MAN-22) serves HTTP on a listener the host proxies to, and MUST accept
connections on it before it sends the `plugin.initialize` reply. Where the
listener is depends on the platform, and the host reads it from the same
places for every plugin:

* **Unix socket.** The plugin listens on `<NGINX_UI_PLUGIN_DATA_DIR>/http.sock`
  (LIFE-14). There is no `http_socket` member: the host takes exactly this
  path, so a plugin whose data directory path does not fit the platform's
  `sun_path` limit (WIRE-11) cannot serve the capability there and SHOULD fail
  the handshake with an internal error (`-32603`) that says why. A plugin MUST
  remove a stale socket file left at that path before listening, SHOULD create
  the socket with mode `0600`, and SHOULD remove it when it exits.
* **Loopback TCP.** A plugin that cannot listen on a Unix socket (Windows)
  listens on `127.0.0.1` with a free port and reports it as `http_port` in the
  reply (LIFE-2). A host on such a platform uses `http_port` and treats a reply
  without it as a plugin that is not serving.

Both listeners are reachable by more than the host (any local process can
connect to a loopback port, and a socket file can be misconfigured), so the
host proves itself on every request with a per process secret:

* At every process start the host generates a new random secret of at least 32
  bytes, encoded in a URL safe alphabet, and passes it in the environment
  variable `NGINX_UI_PLUGIN_HTTP_SECRET` to a plugin that declares `http` with
  `listen` `"unix"`. A host MUST NOT reuse a secret across starts, MUST NOT
  pass it to any other plugin and MUST NOT let the plugin process inherit one
  from its own environment.
* The host sends the secret in the request header `X-Nginx-UI-Plugin-Secret`
  on every request it proxies, on the Unix socket and on the loopback port
  alike, WebSocket upgrades included. It MUST remove every copy of that header
  from the request it received from the client before it adds its own.
* The plugin MUST answer `401` to every request that does not carry exactly one
  such header with the matching value, compare it in constant time, and MUST NOT
  log it, echo it or hand it to its request handlers. A plugin SHOULD read the
  variable once and remove it from its environment so that child processes do
  not inherit it. A plugin that finds the variable unset MUST NOT open the
  listener and SHOULD fail the handshake with an internal error (`-32603`) that
  names the variable.

The host authenticates the person before it proxies a request, removes their
credentials (`Authorization`, `Cookie`) and any client supplied
`X-Nginx-UI-User` and `X-Nginx-UI-User-ID`, and sets the two headers to the id
and name of the person. A plugin that checked the secret can rely on those
headers. WebSocket upgrades and streamed responses pass through.

On `plugin.shutdown` (LIFE-10) a plugin SHOULD stop accepting new connections,
let the requests still running finish within the bound of that step, and close
the ones that do not.

## Resource limits

## LIFE-16

A host MAY confine the resources of a plugin process: its memory, its CPU
time or both, with an operating system mechanism such as a Linux cgroup. The
manifest MAY declare what the process needs at most in
`server.resources` (MAN-39); a host that confines processes applies the
smaller of a hint and its own limit for each resource, so a hint can lower a
limit but never raise it, and ignores the hints otherwise. A host that
confines processes SHOULD show the limits in effect for every plugin and
whether they are actually enforced.

A plugin MUST tolerate being killed at any moment, without `plugin.shutdown`
or `plugin.exit`: a process that exceeds its memory limit is killed with
`SIGKILL` by the kernel's out-of-memory killer. It MUST NOT rely on the stop
sequence (LIFE-10) to keep the files of its data directory consistent. The
host handles such an exit as a crash (LIFE-12), so a plugin that keeps
exceeding its limit ends in the error state.

The reference host confines processes on Linux with cgroup v2 only. It
creates `<cgroup root>/nginx-ui/plugins/<plugin id>` before starting a
process, writes the limits that apply, `memory.max` together with
`memory.swap.max` `0` so a limited process cannot swap, and `cpu.max` as a
quota of `cpu_percent` × 1000 µs per 100 000 µs period (100 being one core),
starts the process inside the group and removes the group once the process
exited. Without any limit it creates no group. Its settings `MemoryLimitMB`
and `CPUPercent` (`0` meaning unlimited) are the host limits, and
`CgroupRoot` (`/sys/fs/cgroup` by default) names the mount point. Where
cgroup v2 is missing or its files cannot be written (another operating
system, a process without the privilege, a container without delegation)
the processes run without limits and the plugin info reports the limits as
not enforced.

## LIFE-19

A host SHOULD show `server.resources.recommended_memory_mb` (MAN-39) wherever
a plugin can be chosen or installed, such as a marketplace and the details of
a plugin. It SHOULD warn before the install, and on the installed plugin,
when the memory the host runs with is below the value: the total memory of
the machine, or the memory limit of the container when the host runs in one.
A host MUST NOT refuse to install or run a plugin because of this value,
which is advice and not a limit (LIFE-16).

## LIFE-20

A host MUST NOT run two conflicting plugins (MAN-42) at the same time. Two
plugins conflict when either of them lists the other in `conflicts`.

* **Enable.** When an enabled plugin conflicts with the plugin being enabled,
  the host MUST refuse with an error that names the enabled plugin. When the
  caller asks to replace the conflicting plugins, the host disables them
  first, as a normal disable does including the plugins that depend on them,
  and then enables the plugin.
* **Install with enable.** A package that is installed and enabled in one step
  (upload, catalog install, automatic install, installing a dependency) is
  installed, but a plugin that conflicts with an enabled plugin stays
  disabled, unless the caller asks to replace the conflicting plugins. An
  upgrade keeps the plugin enabled only when no enabled plugin conflicts with
  the new version, since an upgrade can add a `conflicts` entry.
* **Startup.** The host goes through the enabled plugins in its usual start
  order. A plugin that conflicts with one that stays enabled is not started:
  the host disables it, so it stays off after a restart and none of its pages
  or content are served, and records an error that names the other plugin.
* **Synchronised hosts.** A host that mirrors the state of another host
  (PKG-18) follows that state: enabling a plugin there replaces the
  conflicting plugins.
