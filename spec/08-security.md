# 08. Security

## Trust boundary

## SEC-1

A `server` plugin process runs with the same operating system privileges as
the host process that spawned it. The wire protocol (`spec/03-wire-protocol.md`)
and the permission model below gate access to specific **host functionality**
(the key-value store, stored credentials, notifications, metrics, the core
REST API) — they are not an OS-level sandbox, and this spec does not define
one. A host MUST treat installing a plugin as granting it the same level of
trust as installing any other executable that runs as the host's user, and
SHOULD make that clear to the person installing one. Resource limits
(LIFE-16, SEC-17) bound what a process consumes, not what it may access.

## SEC-2

A host MUST run the plugin process as an unprivileged user consistent with
how it runs its own process, MUST NOT elevate a plugin's privileges beyond
its own, and MUST scope `NGINX_UI_PLUGIN_DATA_DIR` (LIFE-14) to a directory
that plugin, and no other, can write to.

## Permissions

## SEC-3

A plugin declares the `host.*` capabilities it needs in its manifest's
`permissions` array (MAN-23). The fixed names and what each gates:

| Permission | Gates |
| --- | --- |
| `kv` | `host.kv.get`/`set`/`delete`/`list` |
| `network` | Nothing on the wire — it is declarative, telling the person installing the plugin that its own process (not the host) will make outbound network connections. A host MAY use it for an OS-level firewall policy where one exists, but the wire protocol does not enforce it. Required by the `security.blocklist` and `upstream.discovery` capabilities (MAN-36, MAN-37). |
| `cron` | `host.cron.register`/`unregister` |
| `notify` | `host.notify` |
| `metrics.read` | `host.metrics.snapshot` |
| `core_api` | `registry.coreHttp` in the browser webapp contract (WEB-7) — unrelated to any `host.*` JSON-RPC method. |
| `mcp` | Nothing the plugin calls — it gates the host publishing the plugin's `mcp` tools to MCP clients (SEC-13). Required by the `mcp` capability (MAN-33). |
| `cert.deploy` | Nothing the plugin calls — it gates the host sending certificates and their private keys to the plugin in `deploy.push` (SEC-14). Required by the `cert.deploy` capability (MAN-35). |
| `log.read` | Nothing the plugin calls — it gates the host streaming the nginx access log lines to the plugin in `log.push` (SEC-16). Required by the `log.sink` capability (MAN-38). |
| `credentials.read:<kind>` | `host.credentials.get` for that specific `kind` only. |

`host.log`, `host.settings.get` and `host.i18n.locale` require no
permission at all (HOST-3, HOST-7, HOST-8).

## SEC-4

A host MUST check the permission set it actually granted
(`plugin.initialize.params.permissions`), not the manifest's requested set,
for every gated call (HOST-1). This is what makes SEC-9 (upgrade
re-approval) meaningful: a manifest can ask for more, but the process does
not receive more until a person approves it.

## SEC-5

A host MUST NOT grant a permission a plugin's manifest does not request. A
plugin calling a gated method without having declared the matching
permission at all (not just without having it approved) is a defect in the
plugin, and a host MAY refuse to install a plugin whose behavior it can
detect does not match its declared permissions, though this is not required
on the wire.

## SEC-6

`network_hosts` (MAN-1 table) is declarative and informational: a host MAY
show it to the person installing the plugin as "this plugin intends to
contact: ...", and MAY enforce it with an OS-level allowlist where the
platform supports one, but this spec does not require enforcement.

## Credential and secret handling

## SEC-7

A value that is a credential (a `dns01` provider's `configuration.credentials`
entry, a `type: "secret"` settings field, anything delivered through
`host.credentials.get`, the private key `deploy.push` carries) MUST NOT
appear in:

* an error `message` (WIRE-5) — use `data.field` to name the offending field
  instead of quoting its value;
* a `host.log` call's `message` or `fields`;
* stderr output, if the plugin logs there directly.

A plugin SHOULD redact a credential even when only reporting *that* a value
was provided (e.g. logging its length or a masked form) rather than logging
nothing at all about it, so operators can still debug "did the value even
arrive" without the value itself ever being recoverable from a log.

## SEC-8

A host MUST NOT send a stored secret's real value back to a browser client
for display. When rendering a settings form or a credential form with an
existing `type: "secret"` value, the host MUST substitute a fixed
placeholder, and MUST treat that exact placeholder, if resubmitted
unchanged, as "keep the existing stored value" rather than as a literal new
value to store.

## SEC-9

An upgrade that adds a permission beyond what was previously approved for a
plugin MUST NOT take effect silently. A host MUST require explicit
re-approval (the person confirms the new permission set) before the upgraded
plugin process receives the new permission at runtime (SEC-4); until then it
MUST keep running, if at all, with only the previously approved set. A host
SHOULD fingerprint the approved permission set (e.g. a hash of the sorted
list) so it can detect a change cheaply without storing the whole set
redundantly.

## SEC-10

Values exported into the plugin process's own environment for the duration
of one call (for example, a lego-style provider reading `MYDNS_API_TOKEN`
from `os.Environ()`) MUST be restored to their prior state immediately after
the call returns, so that two calls using different credentials of the same
kind never observe each other's values, and MUST NOT be written to any
persistent location by that mechanism.

## SEC-13

Granting `mcp` lets every MCP client the host authorizes run the plugin's
tools (`spec/14-capabilities-mcp.md`). A host MUST NOT publish a plugin's
tools unless `mcp` is in the granted permission set (SEC-4), MUST apply its
own MCP authorization to a call before forwarding it (MCP-8), and SHOULD
show the person approving the permission that an AI assistant will be able
to run the plugin's code on their behalf. A plugin MUST treat tool arguments
as untrusted input (MCP-5).

## SEC-14

Granting `cert.deploy` lets the plugin receive the private key of every
certificate a person binds to one of its targets
(`spec/16-capabilities-deploy.md`). A host MUST NOT call `deploy.push`
unless `cert.deploy` is in the granted permission set (SEC-4), MUST send a
certificate only to targets a person bound to it (DEPLOY-11), and MUST tell
the person approving the permission that the plugin receives certificates
and their private keys in order to push them to external targets. A plugin
MUST treat the key as a credential (SEC-7, DEPLOY-7) and MUST NOT keep it
after the call (DEPLOY-8).

## SEC-15

The `security.blocklist` and `upstream.discovery` capabilities let a plugin
decide what nginx serves: which clients it denies and which servers it
proxies to. A host MUST NOT write text a plugin returned into the nginx
configuration verbatim: it MUST parse every address, network, host name,
port and weight first and drop what does not parse (BLOCKLIST-9,
DISCOVERY-9), so an answer cannot add a directive of its own. A host MUST
write the result only into files of its own that a person includes
deliberately (BLOCKLIST-10, DISCOVERY-10), MUST test the configuration
before every reload and MUST keep the previous file when the test fails. A
person relying on a blocklist source trusts its plugin not to deny them, and
a host SHOULD make it easy to see and disable what a source denies.

## SEC-16

Granting `log.read` lets the plugin receive every line nginx writes to the
access logs the host reads (`spec/20-capabilities-logsink.md`): client
addresses, every requested URL with its query string, which may carry a
session id or a token, referers and user agents. A host MUST NOT stream a
line to a plugin unless `log.read` is in the granted permission set (SEC-4),
MUST stream only logs a person allowed the host to read (LOGSINK-8), and
SHOULD tell the person approving the permission that the plugin receives
the access logs, which are personal data in many jurisdictions. A plugin
MUST treat the entries as personal data and as untrusted input (LOGSINK-7)
and MUST NOT write them to its own stderr.

## SEC-17

A host that confines plugin processes (LIFE-16) MUST apply the limits a
person configured for the host even when a manifest hint asks for more, MUST
NOT let a plugin choose the location or the name of its resource group, and
MUST NOT fail to run a plugin only because the confinement is unavailable
on the platform: it runs the process without limits and reports that the
limits are not enforced. Limits bound the damage a runaway or hostile
plugin does to the availability of the host; they are not a sandbox
(SEC-1).

## Package integrity

## SEC-11

Package extraction safety (no symlinks, no absolute paths, no path
traversal, size and file count limits) is specified in `spec/02-packaging.md`
(PKG-3 through PKG-7); it exists to bound what an untrusted **archive**, as
opposed to an already-running plugin process, can do to the host filesystem
before the manifest has even been validated.

## SEC-12

A host SHOULD verify a package's integrity (for example, a checksum or
signature published alongside the release) before installing it, when such
a signal is available, and MUST make the package's declared `id`, `version`
and `permissions` visible to the person installing it, before installation
completes, regardless of whether an integrity signal is available.
