# NGINX UI Plugin Specification

This repository is the normative specification for the [NGINX UI](https://github.com/0xJacky/nginx-ui)
plugin system: the on-disk package format, the manifest schema, the
JSON-RPC 2.0 wire protocol between the host and a plugin process, the
lifecycle a conformant plugin and host MUST follow, the `dns01` capability,
the host API a plugin process may call, the browser webapp contract, security
and packaging rules, and the naming conventions plugin authors MUST follow.

It exists so that a plugin can be written in any language, against this
document alone, and work with any host that implements it — not only the
reference [nginx-ui](https://github.com/0xJacky/nginx-ui) host, and not only
plugins written with the reference SDKs.

## Who this is for

* **Plugin authors** who want a contract that does not change under them and
  does not require reading Go source.
* **SDK maintainers** ([nginx-ui-plugin-sdk-go](https://github.com/0xJacky/nginx-ui-plugin-sdk-go),
  [nginx-ui-plugin-sdk-web](https://github.com/0xJacky/nginx-ui-plugin-sdk-web),
  or an SDK in a third language) who need the wire format, error codes and
  timing contract to build a helper library against.
* **Host implementers** who want to run the same plugin packages the
  reference host runs, or who are auditing the reference host's behavior
  against a written spec instead of its source.

## Layout

| Path | Contents |
| --- | --- |
| `spec/01-manifest.md` | `plugin.json` schema and validation rules (`MAN-n`) |
| `spec/02-packaging.md` | Archive format, layout, size limits, required files (`PKG-n`) |
| `spec/03-wire-protocol.md` | JSON-RPC 2.0 framing, error codes, message limits (`WIRE-n`) |
| `spec/04-lifecycle.md` | Handshake, liveness, configuration, shutdown (`LIFE-n`) |
| `spec/05-capabilities-dns01.md` | The `dns01` capability's methods (`DNS01-n`) |
| `spec/06-host-api.md` | `host.*` methods, events and cron delivery (`HOST-n`) |
| `spec/07-webapp.md` | Browser bundle contract, slots, registry API (`WEB-n`) |
| `spec/08-security.md` | Permission model, credential handling, trust boundary (`SEC-n`) |
| `spec/09-conformance.md` | Conformance levels: `core`, `dns01`, `webapp` (`CONF-n`) |
| `spec/10-versioning.md` | Spec versioning, `api_version`, upgrade compatibility (`VER-n`) |
| `spec/11-naming.md` | Plugin id and provider code namespaces (`NAME-n`) |
| `spec/methods.json` | Generated table of every JSON-RPC method and its proto rpc (WIRE-9) |
| `proto/nginxui/plugin/v1/` | The proto contract, source of truth for methods and message shapes (WIRE-9) |
| `gen/go/` | Generated Go package `pluginv1`, a Go module of its own |
| `tools/` | Generator of `spec/methods.json` and the consistency tests, a Go module of its own |
| `schema/plugin.schema.json` | JSON Schema (draft 2020-12) for `plugin.json`, checked against `manifest.proto` |
| `examples/python-dns01/` | Zero-dependency Python 3 reference plugin |
| `vectors/v1/` | Request/response test vectors for SDK authors |

Every requirement is numbered (e.g. `LIFE-3`, `DNS01-7`) so it can be linked
to and cited from an implementation's tests or a conformance report. Keywords
"MUST", "MUST NOT", "SHOULD", "SHOULD NOT" and "MAY" are used as defined by
[RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

## Versioning

This document describes **spec 1.0**, which corresponds to wire protocol
`api_version = 1`. See `spec/10-versioning.md` for the full compatibility
model: how `api_version`, a plugin's own `version` and `min_nginx_ui_version`
relate, and what changes are permitted within spec 1.x without incrementing
`api_version`.

## Source of truth

Method names, message shapes, error codes and the manifest structure are
defined once, in the proto contract under `proto/nginxui/plugin/v1/`
(`spec/03-wire-protocol.md` WIRE-9). The JSON on the wire is the protobuf
JSON mapping of those messages with proto field names (WIRE-10), so a plugin
author can keep working from the JSON examples alone.

Everything else follows from the proto and is checked against it:

* `spec/methods.json` and `gen/go/` are generated from it.
* `schema/plugin.schema.json` is written by hand and tested against
  `manifest.proto` (`schema/README.md`).
* The vectors under `vectors/v1/` are tested to decode into the proto
  messages of their methods.
* The reference host ([`internal/plugin/protocol`](https://github.com/0xJacky/nginx-ui/tree/main/internal/plugin/protocol))
  and the Go SDK keep a verbatim copy of `gen/go` in a `pb` package and test
  their hand-written wire types against it.

Behavior that the proto cannot express, such as ordering, timeouts and
permissions, is specified by the chapters and follows the reference host
([`internal/plugin`](https://github.com/0xJacky/nginx-ui/tree/main/internal/plugin))
and the reference browser runtime
([`app/src/plugin`](https://github.com/0xJacky/nginx-ui/tree/main/app/src/plugin)).
Where the reference host's own implementation choice is not itself part of
the wire contract (for example, a specific timeout value), this spec says so
and marks the behavior as a recommendation rather than a hard requirement.

## Toolchain

The contract is built with [buf](https://buf.build) and the Go protobuf
plugins. buf compiles the proto itself, `protoc` is not needed.

```bash
make tools      # go install buf, protoc-gen-go and protoc-gen-go-grpc (pinned)
make generate   # regenerate gen/go and spec/methods.json
make lint       # buf lint (STANDARD rules) and buf format
make check      # lint, fail on stale generated files, run the Go tests
```

The tools land in `$(go env GOPATH)/bin`, which the Makefile puts on `PATH`.
Lint uses the `STANDARD` rule set with one exception, `SERVICE_SUFFIX`: the
service names (`Plugin`, `Host`, `DNS01`, `HTTP`, `Events`) are part of the
published gRPC paths and stay short. The generated files are committed, since
they are published with the spec; run `make generate` after every change
under `proto/` and commit its output together with the change.

`gen/go` is the Go module `github.com/0xJacky/nginx-ui-plugin-spec/gen/go`
(package `pluginv1`, import path `.../gen/go/nginxui/plugin/v1`). `tools/` is a
separate module that uses it through a `replace` directive and is not meant
to be imported.

## Change process (RFC process summary)

This spec changes by pull request against this repository:

1. **Propose.** Open an issue or a draft PR describing the problem, not just
   the field you want to add. Link the nginx-ui issue or discussion if there
   is one.
2. **Draft.** Write the change as a diff to the relevant `spec/*.md` file(s),
   using the next free requirement number in that file's series (never
   reuse or renumber an existing id — see `spec/10-versioning.md`). When the
   change affects the wire format, change the proto under `proto/` first and
   run `make generate`, then update `schema/plugin.schema.json` and add or
   update a vector under `vectors/v1/` (or a new `vectors/v2/` once
   `api_version` actually changes). `make check` MUST pass.
3. **Reference check.** A change MUST match what at least one real
   implementation does, or MUST be implemented in the reference host and the
   reference SDKs before merge. This spec does not accept speculative fields.
4. **Review.** Two maintainers of nginx-ui or a listed SDK approve the PR.
   A change that breaks an existing MUST requirement needs a version bump per
   `spec/10-versioning.md`, not a silent edit.
5. **Land.** Merge to `main`. A change that adds a new optional field or a
   new capability does not require a new spec version; a change that breaks
   wire compatibility does, and ships as spec 2.0 / `api_version = 2` with
   both versions documented side by side until the old one is retired.

## Related repositories

* [nginx-ui](https://github.com/0xJacky/nginx-ui) — the reference host
* [nginx-ui-plugin-sdk-go](https://github.com/0xJacky/nginx-ui-plugin-sdk-go) — Go SDK for server-side plugins
* [nginx-ui-plugin-sdk-web](https://github.com/0xJacky/nginx-ui-plugin-sdk-web) — TypeScript SDK for browser plugin bundles
* [nginx-ui-plugin-dns01](https://github.com/0xJacky/nginx-ui-plugin-dns01) — the official `dns01` capability plugin

## License

AGPL-3.0. See [LICENSE](LICENSE).
