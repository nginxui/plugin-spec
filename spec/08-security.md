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

A value that is a credential (the value of a `dns01` provider's form field
with `group: "credential"`, a `type: "secret"` settings field, anything
delivered through `host.credentials.get`, the private key `deploy.push`
carries) MUST NOT
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

A host MUST determine the signature state and the trust level of a package
(PKG-21, PKG-22, SEC-18) before installing it, and MUST make the package's
declared `id`, `version` and `permissions`, its trust level and, for a
signed package, the key id of its signer visible to the person installing it
before installation completes, at every trust level. A catalog digest
(PKG-16) is checked next to the signature, never instead of it.

## Package trust

A trust level says who published a package. A host derives it from the key
that signed the package (PKG-19 through PKG-21) and from what the release
keys of the project say about that key (PKG-25 through PKG-27, SEC-25
through SEC-29), never from the place the package came from, and it decides
which installs and updates the host allows. It does not limit what an
installed plugin may do: that is the job of the permission model above, and
SEC-1 applies at every level.

## SEC-18: trust levels

A host derives the trust level of a package from the key that signed its
`plugin.sums`:

| Signer | Level |
| --- | --- |
| A release key of the Nginx UI project, pinned in the host binary | `official` |
| A partner key a release key vouches for: the key of a certificate in the package that passes PKG-27, or a key the partner keyring lists whose entry has not expired (SEC-25, SEC-26, SEC-29); in both cases only while its key id is not revoked (SEC-28) | `verified` |
| The `author_public_key` of the catalog entry the package was downloaded from (PKG-24), or a key the operator added to the host's trusted key list (the reference host's `plugin.trusted_public_keys`) | `community` |
| None: no signature, or an unknown signer (PKG-21) | `unsigned` |

The levels rank `unsigned` < `community` < `verified` < `official`. A key
that appears in more than one row gives the highest of its levels. Only a
release key gives `official`, and only a key that a release key vouches for,
through a certificate or the keyring, gives `verified`, so neither a catalog
nor an operator can raise a key above `community`. The release keys change
only with a release of the host; partner keys are added and revoked without
one (SEC-25).

`official` means the Nginx UI project published the package. `verified` is
reserved for partner organizations of the project, whose keys the project
vouches for with its release key; it names the publisher and makes no claim
that anyone reviewed the source. A host SHOULD show the partner name next to
the level: the name of the keyring entry when the keyring lists the key,
otherwise the name in the certificate (PKG-26). `community` means the
package is signed by a key the host learned from a catalog entry or from its
operator. A package that reaches a host without a catalog entry (an upload,
the offline package directory, a push from another host) has no
`author_public_key` to match, so it is `community` only when its key is on
the host's trusted key list.

## SEC-19: the catalog label

The `trust` member of a catalog entry (PKG-24) is informational. A host MAY
use it to label entries and to filter them before it downloads anything, and
MUST NOT grant or raise a level because of it. The level of a package is the
one the host derives when it inspects and installs the package (PKG-22);
where the two differ, the derived level applies to every decision this
chapter ties to trust.

## SEC-20: developer mode

A host MAY offer a developer mode setting (the reference host's
`plugin.developer_mode`), which MUST be off by default. While it is off, or
when the host offers none, a host MUST NOT install an unsigned package from
any source: an upload, the offline package directory, a catalog download or
a push from another host. While it is on, a host MAY install an unsigned
package, and SHOULD show the person installing it that its publisher cannot
be confirmed.

Developer mode does not admit an invalid package (PKG-21), does not lift the
community policy (SEC-21) and does not make an unsigned plugin eligible for
automatic updates (SEC-22). It gates installs only: this spec does not
require a host to stop a plugin installed while developer mode was on once
the mode is switched off.

## SEC-21: community packages

A host MUST NOT install a `community` package unless its community policy
allows community plugins (the reference host's
`plugin.allow_community_plugins`), and a person MUST confirm the install
after the host showed them that the package is signed by its author, not by
the Nginx UI project or a partner. A host MAY count the confirmation given
on a cluster controller for the nodes it pushes the same version to, and MAY
count placing a package in the offline package directory as the
confirmation of the operator who put it there.

## SEC-22: updates

A host that updates plugins automatically MUST do so only for a plugin whose
installed trust (SEC-24) is `official` or `verified`, and MUST refuse an
automatic update whose package derives a level that ranks below the
installed trust of the plugin (a downgrade). When a person starts the
update, the host MUST show them that the package ranks below the installed
trust before it installs the package, so a developer can replace an official
plugin with their own build while developer mode is on. An update is any
install that replaces the package of an installed plugin, and SEC-9 applies
to it as well.

## SEC-23: installs a host starts on its own

A host that installs a plugin which is not installed yet, without a person
asking for that plugin (such as the reference host's automatic install of
the DNS-01 plugin), MUST accept only an `official` package for it.

## SEC-24: recorded trust

A host MUST record, for every installed plugin, the trust level it derived
when it installed the package and the key id of the signer (the 16
hexadecimal digits minisign prints, none for an unsigned package), MUST base
SEC-22 on the recorded level, and MUST make both visible to the person
managing plugins. For a `verified` plugin a host SHOULD record and show the
partner name as well (SEC-18).

## Partner keys

A partner key reaches a host in two ways, both anchored in the release keys
of the project, and neither needs a release of the host. A certificate
(PKG-25 through PKG-27) travels inside every package the partner signs,
needs no network and makes the package `verified` on a host that has never
heard of the partner. The partner keyring, a document the project signs with
a release key and publishes next to the official catalog, lists partner keys
and revoked key ids and reaches every host that refreshes its catalogs, so a
revocation takes effect without waiting for a certificate to expire.

A partner is onboarded as follows. It creates a minisign key pair and sends
its public key to the maintainers. The maintainers issue a certificate: they
sign the public key with a release key, with the partner name and, as
SEC-29 recommends, an expiry date in the trusted comment (PKG-26), and hand
the partner
`plugin.partner` and `plugin.partner.minisig`, which it puts into every
package it signs (PKG-23). They also list the key in the keyring. Before the
certificate expires they issue a new one, which the partner ships with its
next packages.

A partner is offboarded, or a compromised key withdrawn, by listing its key
id in `revoked` and publishing the keyring. From its next refresh on, every
host the keyring reaches stops giving that key `verified`, whether a package
carries a certificate for it or not. A revoked key id stays in the list; a
partner that signs again gets a new key and a new certificate. A host that
no longer refreshes, such as a node cut off from the network, keeps what it
has: it honors a certificate until its expiry date, and a keyring entry
until the `expires` of the entry or, for an entry without one, for as long
as it keeps that keyring. A certificate without an expiry date stays valid
there for good. The lifetime the maintainers give a certificate therefore
bounds the damage on such a host.

## SEC-25: the partner keyring

The partner keyring is a JSON document (SEC-26) that the Nginx UI project
publishes as `v1/partners.json` next to the official catalog `v1/index.json`,
together with `v1/partners.json.minisig`, a minisign signature of the exact
bytes of the document by a release key. For the reference host these are
`https://plugins.nginxui.com/v1/partners.json`
and the same URL with `.minisig` appended. The signature has the text form of
PKG-20, and a host MUST verify it with its release keys the way PKG-20
describes before it parses the document. Its comments carry no meaning.

Only the keyring of the official catalog counts. A host MUST fetch it from
that location, which it knows independently of the catalog sources a person
configures, and MAY fetch it through the mirror or proxy it uses for the
official catalog, since the signature, not the transport, authenticates the
document. A host MUST NOT look for a keyring next to any other catalog
source: a custom source cannot publish partners, and a document is a keyring
only when a release key verifies its signature.

## SEC-26: keyring format

```json
{
  "schema_version": 1,
  "updated_at": "2026-09-26T08:00:00Z",
  "partners": [
    {
      "name": "example-corp",
      "public_key": "untrusted comment: minisign public key 0123456789ABCDEF\nRW...",
      "expires": "2027-09-30"
    }
  ],
  "revoked": ["FEDCBA9876543210"]
}
```

| Member | Type | Required | Meaning |
| --- | --- | --- | --- |
| `schema_version` | integer | yes | `1`, the only layout this spec defines. |
| `updated_at` | string | yes | RFC 3339 time of this version of the document. Every new version carries a later time (SEC-27). |
| `partners` | array | yes | The partner keys, one object each. |
| `partners[].name` | string | yes | The partner name, in the syntax of PKG-26. |
| `partners[].public_key` | string | yes | The minisign public key of the partner, in the text form of `plugin.partner` (PKG-25). |
| `partners[].expires` | string | no | `YYYY-MM-DD`, the last day the entry is valid (SEC-29). Absent means the entry does not expire. |
| `revoked` | array of strings | yes | Revoked partner key ids, each the 16 hexadecimal digits minisign prints for a key (SEC-24). |

[`schema/partners.schema.json`](../schema/partners.schema.json) describes
the document. A host MUST NOT use a document whose `schema_version` it does
not know or whose `updated_at` does not parse, and keeps its cached keyring
instead (SEC-27). A host MUST ignore an entry of `partners` whose `name`,
`public_key` or `expires` does not parse, and a `revoked` value that is not
a key id, and use the rest of the document. A host compares key ids without
regard to case, and ignores members it does not know (VER-4).

A listed key whose entry has not expired needs no certificate: a package it
signs derives `verified` (SEC-18) unless its key id is revoked (SEC-28).

## SEC-27: refresh, rollback and cache

A host MUST fetch the keyring and its signature each time it refreshes its
catalogs, and SHOULD do so even when a person removed the official catalog
from its sources, since revocations reach a host only this way. A host MUST
take a fetched document as its keyring only when:

1. a release key verifies its signature (SEC-25);
2. SEC-26 allows the host to use it; and
3. its `updated_at` is not earlier than the `updated_at` of the keyring the
   host has cached. An older document may lack a revocation the cached one
   has, so a host MUST refuse it (rollback).

A host MUST store the keyring it took, with its signature, on disk in place
of the one it cached before, and MUST load the cached keyring when it
starts, so that a restart or a node without network keeps it. It SHOULD
verify the signature again when it loads the cached keyring. When a fetch
fails, or the host refuses the fetched document, the host MUST keep using
the cached keyring and SHOULD report the failure. A host that has never
obtained a keyring has no keyring entries and no revocations: only
certificates give `verified` there.

The project MUST give every new version of the keyring a later `updated_at`
than the version before it.

## SEC-28: revocation

A partner key whose key id the keyring of the host lists in `revoked` MUST
NOT give `verified`, whatever vouches for it: a certificate that passes
every other step of PKG-27, or an entry of `partners` in the same keyring.
Revocation takes precedence over both.

Revocation withdraws the partner level only. It does not affect a release
key, whose set changes only with a release of the host, and a revoked key
that is the `author_public_key` of the catalog entry or on the operator's
trusted key list still gives `community` (SEC-18), with the confirmation
SEC-21 requires.

A host applies revocation whenever it derives a level (PKG-22). Revocation
does not change the level a host recorded for an installed plugin (SEC-24),
but no update signed by the revoked key derives `verified` any more, so an
automatic update to it is refused as a downgrade (SEC-22). A host SHOULD show
a person managing plugins that the signer of an installed `verified` plugin
is revoked.

## SEC-29: expiry

Both a certificate and a keyring entry may carry an expiry date: the
`expires` field of the trusted comment of a certificate (PKG-26) and the
`expires` member of a keyring entry (SEC-26). Each names the last day the
certificate or the entry is valid, on the UTC calendar. A certificate or an
entry without a date does not expire: revocation ends it (SEC-28), and an
entry also ends when a later keyring drops it. For one that carries a date, a host MUST evaluate the date against the current
time of its own clock, converted to UTC: the certificate or entry is valid
while the UTC date of the clock is on or before the named day, that is
through 23:59:59 UTC of that day, and has expired from 00:00:00 UTC of the
following day. A date that is not a date of the calendar, such as
`2027-02-30`, fails the certificate or the entry.

The project SHOULD give every certificate it issues an expiry date. A long
lifetime, such as several years, is fine. Revocation reaches only the hosts
that refresh the keyring, and a certificate travels in every package the
partner signed; without a date, a node that never refreshes the keyring
trusts a leaked partner key for good.

A host evaluates expiry whenever it derives a level (PKG-22). Like
revocation, expiry does not change the level recorded for an installed
plugin (SEC-24): it bounds which packages a key can still make `verified`,
not which installed plugins stay so.
