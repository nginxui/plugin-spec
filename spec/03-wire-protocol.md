# 03. Wire Protocol

A plugin process speaks bidirectional [JSON-RPC 2.0](https://www.jsonrpc.org/specification)
framed as newline-delimited JSON (NDJSON) over its own **stdin** and
**stdout**. Both sides may originate requests and notifications on the same
pair of streams: the host calls lifecycle and capability methods on the
plugin, and the plugin calls `host.*` methods back (`spec/06-host-api.md`).

Method names and message shapes are defined once, in the proto contract under
`proto/nginxui/plugin/v1/` (WIRE-9). Every `params` and `result` in this spec
is the protobuf JSON mapping of a message from that contract (WIRE-10). The
same services MAY also be served over gRPC (WIRE-11).

## WIRE-1: transport and framing

* The host writes to the plugin process's **stdin**; the plugin writes to its
  own **stdout**. Each direction carries one independent stream of frames.
* A frame is exactly one JSON value followed by a single `\n` (LF). A frame
  MUST NOT itself contain an unescaped newline.
* **`stdout` carries protocol frames only.** A plugin MUST NOT write anything
  else to stdout — no banners, no `print` debugging, nothing. Every human
  readable line (logs, stack traces, startup banners) MUST go to **stderr**,
  which the host captures separately for its own log viewer and never parses
  as protocol.
* Batch requests (a JSON array of request objects, as allowed by base
  JSON-RPC 2.0) are **not** supported. Each line MUST contain exactly one
  JSON object.

## WIRE-2: message shapes

Every frame has `"jsonrpc": "2.0"`. Which other members are present decides
its kind:

| Kind | `id` | `method` | `params` | `result` | `error` |
| --- | --- | --- | --- | --- | --- |
| Request | present, not `null` | present | optional | — | — |
| Notification | absent or `null` | present | optional | — | — |
| Success response | present | absent | — | present | absent |
| Error response | present | absent | — | absent | present |

* `id` MAY be any JSON value that is not `null` in general JSON-RPC 2.0; the
  reference implementations use small non-negative integers assigned by
  whichever side originates the call. An implementation MUST treat `id` as an
  opaque token for matching a response to its request, not as a value with
  meaning beyond that.
* A **request** expects exactly one matching response, success or error.
* A **notification** MUST NOT be answered, by either side.
* `params`, when present, MUST be a JSON object or array as the method
  defines it; every method in this spec uses an object.

## WIRE-3: message size limit

A single frame, including its trailing `\n`, MUST NOT exceed **4 MiB**
(`4 * 1024 * 1024` = 4,194,304 bytes). An implementation MUST treat a larger
incoming frame as fatal to the connection: it MUST stop processing further
frames on that stream and MUST close the connection. This bounds memory use
against a runaway peer on either side.

## WIRE-4: concurrency and ordering

Requests MAY be sent and answered concurrently and out of order. A peer MUST
match every response to its pending request by `id`, and MUST NOT assume
responses arrive in the order requests were sent. A peer MAY process
requests it receives concurrently, or MAY process them strictly in the order
received — the wire format supports either, and this spec does not mandate
one over the other. `spec/04-lifecycle.md` notes the one place order *does*
matter: `plugin.initialize` MUST complete before any other method is sent in
either direction.

## WIRE-5: error object

An error response's `error` member is:

```json
{ "code": -32601, "message": "method not found: dns01.check", "data": null }
```

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `code` | integer | yes | One of the codes in WIRE-6, or a method-specific negative integer this spec does not define. |
| `message` | string | yes | Short, human readable, MUST NOT contain a credential value or other secret (see SEC-7). |
| `data` | any | no | Structured detail. `spec/01-manifest.md`/`spec/08-security.md` define `data.field` for `CodeInvalidConfig`. |

The proto contract names this object `nginxui.plugin.v1.PluginError` and
types `data` as `google.protobuf.Struct`, a JSON object. A sender SHOULD make
`data` an object. A bridge that relays a non-object `data` over gRPC wraps it
as `{ "value": <data> }`.

## WIRE-6: error codes

| Code | Name | Meaning |
| --- | --- | --- |
| `-32700` | Parse error | The frame was not valid JSON. |
| `-32600` | Invalid request | The frame was valid JSON but not a valid JSON-RPC 2.0 message (WIRE-2). |
| `-32601` | Method not found | No handler is registered for `method` at all. |
| `-32602` | Invalid params | `params` failed to decode into the shape the method expects. |
| `-32000` | Internal error | The handler ran and failed for a reason not covered by a more specific code. Any error a handler returns that is not one of the codes below MUST be reported as `-32000`. |
| `-32001` | Permission denied | The caller invoked a `host.*` method that its granted permissions do not cover. See `spec/08-security.md`. |
| `-32002` | Unsupported | The method is a capability method (`spec/05-capabilities-dns01.md`, `http.handle`) the plugin declares the capability for but has not implemented this particular optional method of. |
| `-32003` | Invalid config | A credential or setting failed validation. `data` SHOULD be `{ "field": "<name>" }` naming the offending field. |

A conformant implementation MUST use `-32601`, `-32602` and `-32700` exactly
as defined by base JSON-RPC 2.0, and MUST use `-32000`, `-32001`, `-32002`
and `-32003` exactly as defined above. An implementation MUST NOT invent a
new code in the `-32000`..`-32099` reserved server-error range without a
change to this spec.

## WIRE-7: unknown members

A receiver MUST ignore JSON members it does not recognize, on any message.
This is how this spec adds optional fields (`spec/10-versioning.md`) without
breaking older peers.

## WIRE-8: connection lifetime

The connection is exactly the plugin process's stdin/stdout pipes. It ends
when the process exits, when either side closes its end, or when WIRE-3 is
violated. There is no reconnection within a single process lifetime; a new
connection means a newly spawned process and a fresh `plugin.initialize`
handshake (`spec/04-lifecycle.md`).

## WIRE-9: the proto contract

The proto files under `proto/nginxui/plugin/v1/` (package
`nginxui.plugin.v1`) are the single source of truth for method names and
message shapes. The chapters of this spec describe behavior; when a chapter
and the proto disagree on a method name, a field name or a field type, the
proto wins and the chapter is wrong.

| File | Contents |
| --- | --- |
| `options.proto` | The `rpc_name` and `notification` method options |
| `lifecycle.proto` | Service `Plugin`: `plugin.*` (`spec/04-lifecycle.md`) |
| `host.proto` | Service `Host`: `host.*` (`spec/06-host-api.md`) |
| `dns01.proto` | Service `DNS01`: `dns01.*` (`spec/05-capabilities-dns01.md`) |
| `http.proto` | Service `HTTP`: `http.handle` |
| `events.proto` | Service `Events`: `events.on` |
| `errors.proto` | `PluginError`, `InvalidConfigData` and the `ErrorCode` enum (WIRE-5, WIRE-6) |
| `manifest.proto` | `Manifest`, the shape of `plugin.json` (`spec/01-manifest.md`) |

* Every rpc sets the method option `(nginxui.plugin.v1.rpc_name)`. Its value
  is the JSON-RPC `method`, e.g. `dns01.present`, and is unique across the
  package.
* An rpc with `(nginxui.plugin.v1.notification) = true` is sent as a
  notification (WIRE-2). Its response message is empty and never sent on
  stdio.
* The host serves the `Host` service and the plugin calls it
  (`plugin_to_host`). The plugin serves every other service and the host calls
  it (`host_to_plugin`).
* Request and response messages are named `<Service><Method>Request` and
  `<Service><Method>Response`. Methods that share a shape today
  (`dns01.present` and `dns01.cleanup`, `host.kv.get` and `host.kv.delete`)
  still get one message each so they can evolve separately.
* The proto evolves like any protobuf API within one `api_version`: add fields
  and rpcs, never renumber, retype or reuse a field. A breaking change is a
  new package (`nginxui.plugin.v2`) and a new `api_version`
  (`spec/10-versioning.md`).

`spec/methods.json` is generated from the compiled descriptors by
`tools/methods` (`make generate`). For every rpc it lists `rpc_name`,
`service`, `method`, `full_method` (the gRPC path), `request`, `response`,
`notification` and `direction`. `make check` fails when the file is stale,
and the tests of `tools/methods` assert that every method used in
`vectors/v1/` is an rpc with the same direction and kind, and that every
vector payload decodes into its message.

A plugin author does not need to read the proto: the JSON in these chapters
and the vectors are complete. The proto serves SDK and host implementers and
code generation.

## WIRE-10: JSON mapping

The `params` of a request or notification is the
[protobuf JSON mapping](https://protobuf.dev/programming-guides/json/) of the
rpc's request message, and the `result` of a success response is the mapping
of its response message, under these rules:

* Member names are the proto field names, e.g. `effective_fqdn`. A sender
  MUST NOT use the lowerCamelCase names the mapping also allows.
* An absent member and a member holding its default value (`""`, `0`,
  `false`, `[]`, `{}`, `null`) mean the same. A sender MAY omit a default
  value; a receiver MUST treat an omitted member as its default.
* No field is a 64-bit integer, because the mapping encodes those as JSON
  strings. Counts, ports and durations are `int32`. The event timestamp `ts`
  is `uint32`, so it stays a JSON number past 2038. Every identifier is a
  string.
* Free-form JSON uses the well-known types: `google.protobuf.Struct` for a
  JSON object (settings values, `options`, `fields`, error `data`),
  `google.protobuf.Value` for any JSON value (kv values, event `data`,
  `details`, `snapshot`, a setting's `default`) and
  `google.protobuf.ListValue` for a JSON array (the values of HTTP
  `headers`). Numbers inside them are IEEE 754 doubles, exact for integers up
  to 2^53.
* A `bytes` field (`body_base64`) is a standard base64 string with padding.
  On gRPC it carries the raw bytes.
* Closed sets of strings such as log levels stay `string` fields rather than
  enums, so the JSON carries the plain values.
* A method whose request message is empty MAY be sent without `params`. A
  method whose response message is empty replies with `{}`.
* Unknown members are ignored (WIRE-7).

## WIRE-11: gRPC transport and error mapping

A plugin that lists `grpc` in `transports` (`spec/04-lifecycle.md`) serves the
same services over gRPC. The gRPC paths are the `full_method` values of
`spec/methods.json`, e.g. `/nginxui.plugin.v1.DNS01/Present`. Every rpc is
unary; a notification rpc returns its empty response at once and the caller
ignores it.

A failed gRPC call carries the status code from the table below, a status
message equal to the error `message`, and a `nginxui.plugin.v1.PluginError`
with the JSON-RPC `code` and `data` attached as a status detail
(`google.rpc.Status.details`). A receiver that finds a `PluginError` detail
MUST use its `code`; the status code is the fallback for a peer that attached
none.

| JSON-RPC code | Name | gRPC status |
| --- | --- | --- |
| `-32700` | Parse error | `INVALID_ARGUMENT` |
| `-32600` | Invalid request | `INVALID_ARGUMENT` |
| `-32601` | Method not found | `UNIMPLEMENTED` |
| `-32602` | Invalid params | `INVALID_ARGUMENT` |
| `-32000` | Internal error | `INTERNAL` |
| `-32001` | Permission denied | `PERMISSION_DENIED` |
| `-32002` | Unsupported | `UNIMPLEMENTED` |
| `-32003` | Invalid config | `INVALID_ARGUMENT`, with `data.field` (`InvalidConfigData`) in the detail |

`-32700` and `-32600` do not arise on gRPC itself, where the gRPC runtime
rejects malformed messages; they are listed for bridges that relay a JSON-RPC
error. Without a `PluginError` detail a receiver maps the status back:

| gRPC status | JSON-RPC code |
| --- | --- |
| `OK` | success, the response message is the `result` |
| `INVALID_ARGUMENT` | `-32602` |
| `UNIMPLEMENTED` | `-32002` when the method is an rpc of the contract, `-32601` otherwise |
| `PERMISSION_DENIED`, `UNAUTHENTICATED` | `-32001` |
| any other status | `-32000` |
