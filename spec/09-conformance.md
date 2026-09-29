# 09. Conformance

This spec defines twelve conformance levels: `core`, `dns01`, `webapp`,
`notify`, `probe`, `mcp`, `storage`, `cert.deploy`, `content`,
`security.blocklist`, `upstream.discovery` and `log.sink`. A plugin or a host declares
which level(s) it
targets; a level is satisfied only when every requirement it lists holds,
not merely most of them. Levels are additive: every level other than `core`
includes everything `core` requires.

## CONF-1: level `core`

Every plugin, regardless of capability, MUST satisfy `core`. A host MUST
satisfy `core` to run any plugin at all.

| Section | Requirements |
| --- | --- |
| Manifest | MAN-1 through MAN-14, MAN-18 through MAN-30, MAN-39, MAN-40 (every manifest-level requirement except the `webapp`-specific MAN-15/16/17/41, which only apply to a plugin that declares `webapp`, and the capability blocks MAN-31 through MAN-38, which belong to their levels) |
| Packaging | PKG-1 through PKG-13, PKG-19 through PKG-23 and PKG-25 through PKG-27 for every package; PKG-14 through PKG-17 and PKG-24 for a catalog publisher and for a host that installs from a catalog; PKG-18 only for a host that installs plugins on other hosts |
| Wire protocol | WIRE-1 through WIRE-10; WIRE-11, WIRE-12 and CONF-7 only for a plugin that lists `grpc` in `transports` |
| Lifecycle | LIFE-1 through LIFE-16; LIFE-17 for a host that has its own proxy configuration, LIFE-18 for a plugin that declares `http` with `listen` `"unix"` and for the host that runs it |
| Host API | HOST-1 through HOST-19, limited to the methods the plugin actually calls or subscribes to — a plugin that never calls `host.cron.register` is not tested against HOST-10, but MUST still handle `host.log`/`host.settings.get`/`host.i18n.locale` correctly if it uses them |
| Security | SEC-1 through SEC-12, SEC-18 through SEC-30 |
| Versioning | VER-1 through VER-6 |
| Naming | NAME-1 through NAME-6, NAME-9 |

## CONF-2: level `dns01`

A plugin declaring `"dns01"` in `capabilities` MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-21 (the `dns01` capability declaration rules) |
| Capability | DNS01-1 through DNS01-18 |

`dns01.validate`, `dns01.options` and `dns01.check` are individually
optional (a plugin MAY reply `-32002` to any of them), but `dns01.present`
and `dns01.cleanup` MUST both be implemented for a plugin to claim this
level at all.

## CONF-3: level `webapp`

A plugin declaring a `webapp` block MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-15, MAN-16, MAN-17, MAN-41 (the `webapp` block rules) |
| Webapp | WEB-1 through WEB-15 |

A plugin whose `webapp` block declares only `pages` (no `bundle_path`) is
exempt from WEB-1 through WEB-8 and WEB-13 through WEB-15 (the bundle contract) but MUST still satisfy
WEB-10 and WEB-11 (the iframe page contract).

## CONF-4: host conformance

A host implementation claims a level the same way: `core` is mandatory, and
`dns01`, `webapp`, `notify`, `probe`, `mcp`, `storage`, `cert.deploy`,
`content`, `security.blocklist`, `upstream.discovery` and `log.sink` apply
only if the host intends to run plugins of that kind at all. For the capability levels,
the host side is the requirements a chapter marks as host behavior
(NOTIFY-9 through NOTIFY-11, PROBE-7 and PROBE-8, MCP-7 and MCP-8, SEC-13,
STORAGE-12 through STORAGE-14, DEPLOY-10 through DEPLOY-12, SEC-14,
CONTENT-1, CONTENT-4, CONTENT-5, CONTENT-8, CONTENT-9, BLOCKLIST-8 through
BLOCKLIST-11, DISCOVERY-8 through DISCOVERY-11, SEC-15, LOGSINK-8 through
LOGSINK-11, SEC-16). A host that confines plugin processes also satisfies
LIFE-16 and SEC-17.
A host MAY legitimately support `core` and `dns01` but not `webapp`
(e.g. a headless installation with no browser UI) or vice versa; it MUST NOT
claim a level while silently skipping one of that level's MUST requirements.

## CONF-5: reporting

A conformance report (for a plugin, an SDK, or a host) SHOULD list the
specific requirement ids checked and their pass/fail outcome, not just a
level name, so a reader can see exactly what was and was not verified. The
`vectors/v1/` directory exists to make at least the wire-level requirements
(WIRE-\*, LIFE-\*, DNS01-\*, NOTIFY-\*, PROBE-\*, MCP-\*, STORAGE-\*,
DEPLOY-\*, BLOCKLIST-\*, DISCOVERY-\*, LOGSINK-4, and the `host.*` methods in HOST-\*)
mechanically checkable without a live host or a live plugin on the other
end.

## CONF-6: partial implementations

A component that implements a strict subset of a level's MUST requirements
is not conformant to that level and MUST NOT claim to be. It MAY describe
itself as "implements a subset of spec 1.0 `core`" or similar, naming which
requirements it does not meet, rather than claiming a level it only mostly
satisfies.

## CONF-7: transports

A plugin that lists `grpc` in `transports` MUST satisfy every requirement of
the levels it claims on both transports, stdio and gRPC (WIRE-11), and in
addition:

* **WIRE-11** — the endpoint it reports accepts a connection and answers
  `plugin.ping` over gRPC right after the handshake.
* **TRANSPORT-1** — results are identical under both transports: the same
  request produces the same result, or the same error `code`, `message` and
  `data`, whether it travels on stdio or on gRPC. A conformance run compares
  normalized JSON (a result decoded into its message and encoded again, an
  error as its `code`, `message` and `data`) of at least `dns01.options` and
  `dns01.validate` with an empty `config` for a `dns01` plugin,
  `notify.validate` with an empty `config` for a `notify` plugin, `mcp.call`
  of an unknown tool for an `mcp` plugin, `storage.validate` with an empty
  `config` for a `storage` plugin, `deploy.validate` with an empty `config`
  for a `cert.deploy` plugin, `blocklist.fetch` and `discovery.resolve` with
  an empty `config` for a `security.blocklist` or `upstream.discovery`
  plugin whose first entry declares a required field (without one the call
  may reach a live source whose answer changes between the two calls), and
  of a method outside the
  contract, for which only `code` and `data` are compared because the
  message names the method as each transport spells it. A streaming rpc
  (WIRE-12) exists on gRPC only and is not compared.

A plugin that does not list `grpc` is checked on stdio only; a host is never
required to use gRPC.

## CONF-8: level `notify`

A plugin declaring `"notify"` in `capabilities` MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-31 (the `notify` capability declaration rules) |
| Capability | NOTIFY-1 through NOTIFY-8 |

`notify.validate` is optional (a plugin MAY reply `-32002`), but
`notify.send` MUST be implemented for a plugin to claim this level.

## CONF-9: level `probe`

A plugin declaring `"probe"` in `capabilities` MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-32 (the `probe` capability declaration rules) |
| Capability | PROBE-1 through PROBE-6 |

## CONF-10: level `mcp`

A plugin declaring `"mcp"` in `capabilities` MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-23, MAN-33 (the `mcp` capability declaration rules) |
| Capability | MCP-1 through MCP-6 |
| Security | SEC-13 |

## CONF-11: level `storage`

A plugin declaring `"storage"` in `capabilities` MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-34 (the `storage` capability declaration rules) |
| Capability | STORAGE-1 through STORAGE-11 |

`storage.validate` is optional (a plugin MAY reply `-32002`), but
`storage.put`, `storage.get`, `storage.list` and `storage.delete` MUST all
be implemented for a plugin to claim this level.

## CONF-12: level `cert.deploy`

A plugin declaring `"cert.deploy"` in `capabilities` MUST additionally
satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-23, MAN-35 (the `cert.deploy` capability declaration rules) |
| Capability | DEPLOY-1 through DEPLOY-9 |
| Security | SEC-7, SEC-14 |

`deploy.validate` is optional (a plugin MAY reply `-32002`), but
`deploy.push`, including its dry run, MUST be implemented for a plugin to
claim this level.

## CONF-13: level `content`

A plugin declaring a `content` block MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-18 (the `content` block rules) |
| Content | CONTENT-1 through CONTENT-3, CONTENT-6 and CONTENT-7 |

The level is checked statically, from the files of the package: a
conformance run of a plugin without `server` starts no process and runs only
these checks (and the webapp checks when the plugin declares a `webapp`).

## CONF-14: level `security.blocklist`

A plugin declaring `"security.blocklist"` in `capabilities` MUST
additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-23, MAN-36 (the `security.blocklist` capability declaration rules) |
| Capability | BLOCKLIST-1 through BLOCKLIST-7 |

## CONF-15: level `upstream.discovery`

A plugin declaring `"upstream.discovery"` in `capabilities` MUST
additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-23, MAN-37 (the `upstream.discovery` capability declaration rules) |
| Capability | DISCOVERY-1 through DISCOVERY-7 |

## CONF-16: level `log.sink`

A plugin declaring `"log.sink"` in `capabilities` MUST additionally
satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-23, MAN-38 (the `log.sink` capability declaration rules) |
| Wire protocol | WIRE-11, WIRE-12 |
| Capability | LOGSINK-1 through LOGSINK-7 |
| Security | SEC-16 |

The level needs the gRPC transport: a `log.sink` plugin that does not list
`grpc` in `transports` cannot claim it (LOGSINK-4).

## Reference conformance runner

`nginx-ui plugin conformance <path> [--capability dns01|notify|probe|mcp|storage|cert.deploy|security.blocklist|upstream.discovery|log.sink] [--transport stdio|grpc|both] [--timeout 90s]`
starts the plugin under the reference host's own supervisor and host API
and reports every case with the requirement it maps to and the transport it
ran over (CONF-5). `<path>` is a plugin directory or a package. A plugin
without `server` is checked statically only (CONF-13).

`--transport` picks the transports the protocol and capability cases run
over. The default is `both` when the plugin lists `grpc` in `transports` and
`stdio` otherwise. Asking for `grpc` or `both` from a plugin that does not
list `grpc` fails the `WIRE-11` case.

| Id | Transport | Case |
| --- | --- | --- |
| LIFE-1, LIFE-3, LIFE-4 | — | The handshake completes in time with a matching `api_version` and capability set. |
| LIFE-8 | both | `plugin.ping` answers. |
| WIRE-6 | both | A method outside the contract answers `-32601`. On gRPC it is sent to a path outside the contract. |
| WIRE-2 | stdio | A notification for an unknown method is not answered and the connection stays usable. |
| WIRE-4 | both | 20 concurrent `plugin.ping` calls are all answered. |
| WIRE-1 | stdio | The plugin logs to stderr. |
| WIRE-6 | both | Malformed params answer `-32602` or `-32000` instead of hanging: a JSON string for an object on stdio, request bytes that are no valid message on gRPC. |
| WIRE-11 | grpc | The reported endpoint accepts a connection and answers `plugin.ping`. |
| DNS01-4, DNS01-9, DNS01-10, DNS01-11 | both | The `dns01` methods against the manifest's first provider, as in CONF-2. |
| NOTIFY-8 | both | `notify.validate` with an empty `config` against the manifest's first channel answers `-32003` with `data.field` when the channel declares a required field and `{}` otherwise; `-32002` or `-32601` is reported as skipped. `notify.send` is never called, since it would reach the vendor. |
| PROBE-4, PROBE-5 | both | `probe.check` of the manifest's first kind, with an empty `config`, the unroutable target `http://conformance.invalid` and `timeout_seconds` 5, answers within 15 seconds with a known `status` and a non-negative `latency_ms`, or with `-32003` and `data.field`. |
| MCP-6 | both | `mcp.call` of a tool the manifest does not declare answers `-32602`. No declared tool is called, since it may change state. |
| STORAGE-10 | both | `storage.validate` with an empty `config` against the manifest's first backend answers `-32003` with `data.field` when the backend declares a required field and `{}` otherwise; `-32002` or `-32601` is reported as skipped. |
| STORAGE-8 | both | `storage.list` with an empty `config` and an empty `prefix` against the first backend answers within 30 seconds: `-32003` with `data.field` when the backend declares a required field, otherwise a result whose `objects` is an array (absent counts as empty). `storage.put`, `storage.get` and `storage.delete` are never called, since they change or fetch real data. |
| DEPLOY-9 | both | `deploy.validate` with an empty `config` against the manifest's first target kind, as for NOTIFY-8. |
| DEPLOY-6 | both | `deploy.push` with `dry_run: true`, an empty `config` and a throwaway self-signed certificate for `conformance.invalid` answers within 30 seconds: `-32003` with `data.field` when the kind declares a required field, otherwise a result. A real push is never made. |
| BLOCKLIST-5, BLOCKLIST-6 | both | `blocklist.fetch` with an empty `config` against the manifest's first source kind answers within 60 seconds: `-32003` with `data.field` when the kind declares a required field; otherwise a result whose `entries` is an array (absent counts as empty) of entries whose `cidr` parses, or `-32003` naming a field. |
| DISCOVERY-5, DISCOVERY-6 | both | `discovery.resolve` with an empty `config` and the service `nginx-ui-conformance` against the manifest's first provider answers within 30 seconds: `-32003` with `data.field` when the provider declares a required field; otherwise a result whose `targets` is an array (absent counts as empty) of targets with a port between 1 and 65535, or `-32003` naming a field (an unknown service names `service`). |
| LOGSINK-4 | stdio | `log.push` sent on stdio answers `-32601`, the stream has no JSON-RPC form. The case fails as well when the plugin does not list `grpc` in `transports`. |
| LOGSINK-5 | grpc | A `log.push` stream of three `combined` entries for `/var/log/nginx/conformance.log` answers within 10 seconds with `accepted` 3. It is skipped when the run is limited to `--transport stdio`. |
| CONTENT-2, CONTENT-3 | — | Static checks of `content.templates`: the directory exists, holds a template, and every template parses and renders with its default values. |
| CONTENT-6, CONTENT-7 | — | Static checks of `content.locales`: the directory exists, every `.po` file is named after a host language and parses. |
| TRANSPORT-1 | — | Runs when both transports ran; see CONF-7. |
| WEB-1, WEB-5 | — | Static checks of the webapp bundle. |
| WEB-13 | — | Static checks of the chunks: every `webapp.chunks` file exists in the package and is a non-empty `.js` file. |
| LIFE-10 | — | `plugin.shutdown` and `plugin.exit` stop the process in time. |

## Reference linter

`nginx-ui plugin lint <path>` checks a plugin directory or a package against
this spec and tags every finding with the requirement it maps to, so its
output reads as a partial conformance report (CONF-5). The package-level
checks, and what each id means in its output:

| Id | Level | Check |
| --- | --- | --- |
| PKG-1 | warning | A package file name that parses (NAME-9) carries another id or version than its manifest. |
| PKG-2 through PKG-7 | error | Archive layout, entry types, safe paths and size limits. |
| PKG-8 | error or warning | `README.md` is missing (error), `LICENSE` or `CHANGELOG.md` is missing (warning). |
| PKG-9 | error | A file `server.executables` or a path-containing `server.command[0]` declares is missing from the package; a missing executable bit is only a warning. |
| PKG-12 | error | A per-platform package (`<id>-<version>-<goos>-<goarch>.tar.gz`) does not declare exactly its own platform in `server.executables`. |
| PKG-19, PKG-21 | error | `plugin.sums` is present and does not match the files: a line breaks PKG-19, a listed file is missing or has another SHA-256, or a regular file is not listed. Checked whether or not the package is signed. |
| PKG-20 | warning | Only one of `plugin.sums` and `plugin.sums.minisig` is present, so the package counts as unsigned. |
| PKG-25, PKG-26 | warning | Only one of `plugin.partner` and `plugin.partner.minisig` is present, so the package carries no partner certificate. |
| PKG-26, PKG-27 | warning | The partner certificate does not verify: `plugin.partner` is no minisign public key, no release key pinned in the linter's binary verifies `plugin.partner.minisig`, or its trusted comment breaks PKG-26. A host ignores such a certificate. |
| PKG-27 | warning | The trusted comment of the partner certificate carries an expiry date, and the certificate has expired by the UTC date of the linter's clock (SEC-29). A certificate without a date is never reported as expired. |
| PKG-27 | warning | The partner certificate verifies, but `plugin.sums.minisig` is not signed by the key it names, so it gives the package nothing. |
| SEC-18 | warning | `plugin.sums.minisig` does not verify with a key the linter knows: a release key pinned in its binary, or the partner key of a certificate in the package that verifies and has not expired (a certificate without an expiry date does not expire). This is expected for a community plugin, whose key only a catalog entry or an operator names, and for a partner plugin that relies on the keyring alone. |

PKG-12 is only checked for an archive, since a plugin directory has no file
name to compare against. For a plugin directory the directory itself is the
package root of PKG-19. The linter has no catalog to look at, so PKG-13
through PKG-18 and PKG-24 are left to the catalog tooling and the host. It
does not consult the partner keyring either, so the revocation step of
PKG-27 and the keys only the keyring lists (SEC-25 through SEC-28) are left
to the host.

Manifest findings carry the MAN-n id of the rule they break. The capability
blocks of this spec version add these ids:

| Id | Level | Check |
| --- | --- | --- |
| DNS01-18 | error | A provider has no `form`, or its `form` has a field with an empty or duplicate `key` or no `label`, a `group` other than `credential` or `setting`, a `unit` other than `seconds`, a single method, a method with a duplicate name, a key in `fields` that is not a credential field, an empty `values` key, a `values` key that is also a field key without being a credential field that some method lists and no method both lists and sets, two methods with the same fields and the same values, or more than one recommended method. |
| MAN-31, MAN-32, MAN-33 | error | A declared `notify`, `probe` or `mcp` capability has no block or an empty list; `mcp` without the `mcp` permission. |
| NOTIFY-2, PROBE-2 | error | A channel or kind code does not match `^[a-z0-9-]{2,32}$` or is declared twice. |
| NOTIFY-3, PROBE-3 | error | A channel or kind has no `name`. |
| NOTIFY-4 | error | A configuration field has an empty or duplicate `key`, an unknown `type` or no `display_name`. Reported under PROBE-3 for a probe kind. |
| MCP-2 | error | A tool name does not match `^[a-z0-9][a-z0-9_-]{0,47}$`, is declared twice or has no `description`. |
| MCP-3 | error | An `input_schema` whose `type` is not `"object"`. |
| SEC-3 | warning | A `notify` or `probe` plugin does not request `network`. |
| SEC-5 | warning | The `mcp` permission is requested without the `mcp` capability. |
| MAN-34, MAN-35 | error | A declared `storage` or `cert.deploy` capability has no block or an empty list; `cert.deploy` without the `cert.deploy` permission. |
| STORAGE-2, DEPLOY-2 | error | A backend or target kind code does not match `^[a-z0-9-]{2,32}$` or is declared twice. |
| STORAGE-3, DEPLOY-3 | error | A backend or target kind has no `name`, or a configuration field breaks NOTIFY-4. |
| SEC-3 | warning | A `storage` or `cert.deploy` plugin does not request `network`. |
| SEC-5 | warning | The `cert.deploy` permission is requested without the `cert.deploy` capability. |
| MAN-36, MAN-37 | error | A declared `security.blocklist` or `upstream.discovery` capability has no block or an empty list, or the manifest does not request `network`. |
| BLOCKLIST-2, DISCOVERY-2 | error | A source kind or provider code does not match `^[a-z0-9-]{2,32}$` or is declared twice. |
| BLOCKLIST-3, DISCOVERY-3 | error | A source kind or provider has no `name`, a configuration field breaks NOTIFY-4, or a source kind has a `refresh_seconds` between 1 and 59 or below 0. |
| MAN-38 | error | A declared `log.sink` capability without the `log.read` permission. |
| LOGSINK-1 | warning | A `log_sink` block without the `log.sink` capability, which has no effect. |
| LOGSINK-2 | error | `log_sink.batch_size` outside 0 to 4096, or `log_sink.flush_interval_ms` between 1 and 49 or below 0. |
| LOGSINK-3 | error | A `log_sink.formats` entry that is not `combined` or `raw`, or that appears twice. |
| SEC-5 | warning | The `log.read` permission is requested without the `log.sink` capability. |
| MAN-39 | error | A negative `server.resources.memory_mb` or `server.resources.cpu_percent`. |
| MAN-41 | error | `webapp.chunks` without `webapp.bundle_path`, a chunk name that does not match `^[a-z0-9][a-z0-9_-]{0,31}$`, a path that is not a safe relative path ending in `.js`, is the bundle itself or is used by two chunks, or a file that is missing from the package. |
| HOST-18 | warning | `log.paths_changed` in `events` without the `log.files` permission, an event that is never delivered. |
| MAN-40 | error | An `i18n` key that is not a language of the host. |
| CONTENT-1 | error | A manifest without `server` declares `capabilities`, `cron` or `events`. |
| CONTENT-2 | error or warning | `content.templates` is missing, is not a directory or holds no template in `conf/` or `block/` (error); an entry there is not a template (warning). |
| CONTENT-3 | error or warning | A template does not parse or does not render with its default values (error); it has no `name` (warning). |
| CONTENT-6 | error or warning | `content.locales` is missing, is not a directory or holds no `.po` file (error); a `.po` file is named after a language the host does not have (error); another entry is there (warning). |
| CONTENT-7 | error | A `.po` file does not parse. |
