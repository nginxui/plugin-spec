# 03. Wire Protocol

A plugin process speaks bidirectional [JSON-RPC 2.0](https://www.jsonrpc.org/specification)
framed as newline-delimited JSON (NDJSON) over its own **stdin** and
**stdout**. Both sides may originate requests and notifications on the same
pair of streams: the host calls lifecycle and capability methods on the
plugin, and the plugin calls `host.*` methods back (`spec/06-host-api.md`).

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
