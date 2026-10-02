# NGINX UI Plugin Contract

The machine readable contract between [NGINX UI](https://github.com/0xJacky/nginx-ui)
and its plugins: the protobuf definitions of every method and message, the
JSON Schemas of `plugin.json`, the marketplace catalog and the partner
keyring, the generated Go code, and test vectors of the wire protocol.

The host, the SDKs and the official plugins all build on this one copy.
How to write a plugin, what each capability does and how the host behaves
is described in the developer guide:
**[nginxui.com/plugin](https://nginxui.com/plugin/overview)**.

## Layout

| Path | Contents |
| --- | --- |
| `proto/nginxui/plugin/v1/` | The proto contract, source of truth for methods and message shapes |
| `gen/go/` | Generated Go package `pluginv1`, a Go module of its own |
| `gen/methods.json` | Generated table of every JSON-RPC method name and its proto rpc |
| `schema/plugin.schema.json` | JSON Schema (draft 2020-12) for `plugin.json`, checked against `manifest.proto` |
| `schema/catalog.schema.json` | JSON Schema (draft 2020-12) for a marketplace catalog document |
| `schema/partners.schema.json` | JSON Schema (draft 2020-12) for the partner keyring published next to the official catalog |
| `vectors/v1/` | Request and response test vectors for SDK and host authors |
| `examples/python-dns01/` | Zero dependency Python 3 example plugin |
| `tools/` | Generator of `gen/methods.json` and the consistency tests, a Go module of its own |

## Source of truth

Method names, message shapes, error codes and the manifest structure are
defined once, in the proto contract under `proto/nginxui/plugin/v1/`. The JSON
on the wire is the protobuf JSON mapping of those messages with proto field
names, so a plugin author can work from the JSON examples alone.

Everything else follows from the proto and is checked against it:

* `gen/methods.json` and `gen/go/` are generated from it.
* `schema/plugin.schema.json` is written by hand and tested against
  `manifest.proto` (`schema/README.md`).
* The vectors under `vectors/v1/` are tested to decode into the proto
  messages of their methods (`vectors/README.md`).
* The host ([`internal/plugin/protocol`](https://github.com/0xJacky/nginx-ui/tree/dev/internal/plugin/protocol))
  and the Go SDK keep a verbatim copy of `gen/go` in a `pb` package and test
  their hand-written wire types against it. The Rust SDK generates its own
  code from `proto/` and checks its rpc table against `gen/methods.json`.

Behavior the proto cannot express, such as ordering, timeouts and
permissions, is described in the developer guide and implemented by the host
([`internal/plugin`](https://github.com/0xJacky/nginx-ui/tree/dev/internal/plugin))
and its browser runtime
([`app/src/plugin`](https://github.com/0xJacky/nginx-ui/tree/dev/app/src/plugin)).

## Toolchain

The contract is built with [buf](https://buf.build) and the Go protobuf
plugins. buf compiles the proto itself, `protoc` is not needed.

```bash
make tools      # go install buf, protoc-gen-go and protoc-gen-go-grpc (pinned)
make generate   # regenerate gen/go and gen/methods.json
make lint       # buf lint (STANDARD rules) and buf format
make check      # lint, fail on stale generated files, run the Go tests
```

The tools land in `$(go env GOPATH)/bin`, which the Makefile puts on `PATH`.
Lint uses the `STANDARD` rule set with one exception, `SERVICE_SUFFIX`: the
service names (`Plugin`, `Host`, `DNS01`, `HTTP`, `Notify`, `Probe`, `MCP`,
`Storage`, `Deploy`, `Blocklist`, `Discovery`, `LogSink`, `Events`) are part
of the published gRPC paths and stay short. The generated files are
committed; run `make generate` after every change under `proto/` and commit
its output together with the change.

`gen/go` is the Go module `github.com/nginxui/plugin-spec/gen/go`
(package `pluginv1`, import path `.../gen/go/nginxui/plugin/v1`). `tools/` is a
separate module that uses it through a `replace` directive and is not meant
to be imported.

## Changing the contract

Changes land by pull request, together with their implementation in the host
and the SDKs:

1. Change the proto under `proto/` and run `make generate`.
2. Update the schemas and add or update a vector under `vectors/v1/` when the
   wire format changes. `make check` must pass.
3. Describe the change in the developer guide of the nginx-ui repository
   (`docs/plugin/`).

A new optional field or a new capability keeps `api_version` 1. A change that
breaks wire compatibility ships as `api_version` 2, with both versions
supported side by side until the old one is retired.

## Related repositories

* [nginx-ui](https://github.com/0xJacky/nginx-ui): the host
* [plugin-sdk-go](https://github.com/nginxui/plugin-sdk-go): Go SDK for server side plugins
* [plugin-sdk-rust](https://github.com/nginxui/plugin-sdk-rust): Rust SDK for server side plugins
* [plugin-sdk-web](https://github.com/nginxui/plugin-sdk-web): TypeScript SDK for browser plugin bundles
* [plugins](https://github.com/nginxui/plugins): the official catalog, served at plugins.nginxui.com
* [plugin-dns01](https://github.com/nginxui/plugin-dns01): the official `dns01` plugin

## License

AGPL-3.0. See [LICENSE](LICENSE).
