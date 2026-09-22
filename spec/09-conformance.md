# 09. Conformance

This spec defines three conformance levels. A plugin or a host declares
which level(s) it targets; a level is satisfied only when every requirement
it lists holds, not merely most of them. Levels are additive: `dns01` and
`webapp` both include everything `core` requires.

## CONF-1: level `core`

Every plugin, regardless of capability, MUST satisfy `core`. A host MUST
satisfy `core` to run any plugin at all.

| Section | Requirements |
| --- | --- |
| Manifest | MAN-1 through MAN-14, MAN-18 through MAN-30 (every manifest-level requirement except the `webapp`-specific MAN-15/16/17, which only apply to a plugin that declares `webapp`) |
| Packaging | PKG-1 through PKG-13 for every package; PKG-14 through PKG-17 for a catalog publisher and for a host that installs from a catalog; PKG-18 only for a host that installs plugins on other hosts |
| Wire protocol | WIRE-1 through WIRE-10; WIRE-11 and CONF-7 only for a plugin that lists `grpc` in `transports` |
| Lifecycle | LIFE-1 through LIFE-15 |
| Host API | HOST-1 through HOST-16, limited to the methods the plugin actually calls or subscribes to — a plugin that never calls `host.cron.register` is not tested against HOST-10, but MUST still handle `host.log`/`host.settings.get`/`host.i18n.locale` correctly if it uses them |
| Security | SEC-1 through SEC-12 |
| Versioning | VER-1 through VER-6 |
| Naming | NAME-1 through NAME-6, NAME-9 |

## CONF-2: level `dns01`

A plugin declaring `"dns01"` in `capabilities` MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-21 (the `dns01` capability declaration rules) |
| Capability | DNS01-1 through DNS01-13 |

`dns01.validate`, `dns01.options` and `dns01.check` are individually
optional (a plugin MAY reply `-32002` to any of them), but `dns01.present`
and `dns01.cleanup` MUST both be implemented for a plugin to claim this
level at all.

## CONF-3: level `webapp`

A plugin declaring a `webapp` block MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-15, MAN-16, MAN-17 (the `webapp` block rules) |
| Webapp | WEB-1 through WEB-12 |

A plugin whose `webapp` block declares only `pages` (no `bundle_path`) is
exempt from WEB-1 through WEB-8 (the bundle contract) but MUST still satisfy
WEB-10 and WEB-11 (the iframe page contract).

## CONF-4: host conformance

A host implementation claims a level the same way: `core` is mandatory, and
`dns01`/`webapp` apply only if the host intends to run plugins of that kind
at all. A host MAY legitimately support `core` and `dns01` but not `webapp`
(e.g. a headless installation with no browser UI) or vice versa; it MUST NOT
claim a level while silently skipping one of that level's MUST requirements.

## CONF-5: reporting

A conformance report (for a plugin, an SDK, or a host) SHOULD list the
specific requirement ids checked and their pass/fail outcome, not just a
level name, so a reader can see exactly what was and was not verified. The
`vectors/v1/` directory exists to make at least the wire-level requirements
(WIRE-\*, LIFE-\*, DNS01-\*, and the `host.*` methods in HOST-\*)
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
  `dns01.validate` with an empty `config` for a `dns01` plugin, and of a
  method outside the contract, for which only `code` and `data` are compared
  because the message names the method as each transport spells it.

A plugin that does not list `grpc` is checked on stdio only; a host is never
required to use gRPC.

## Reference conformance runner

`nginx-ui plugin conformance <path> [--capability dns01] [--transport stdio|grpc|both] [--timeout 90s]`
starts the plugin under the reference host's own supervisor and host API
and reports every case with the requirement it maps to and the transport it
ran over (CONF-5). `<path>` is a plugin directory or a package.

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
| TRANSPORT-1 | — | Runs when both transports ran; see CONF-7. |
| WEB-1, WEB-5 | — | Static checks of the webapp bundle. |
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
| SEC-12 | warning | A sibling `.minisig` does not verify with the pinned release keys. |

PKG-12 is only checked for an archive, since a plugin directory has no file
name to compare against. The linter has no catalog to look at, so PKG-13
through PKG-18 are left to the catalog tooling and the host.
