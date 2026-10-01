# Test vectors

Each file in `v1/` is one wire-level scenario for plugin API version 1
(`api_version = 1`): a JSON-RPC frame (or two, for a request/response pair)
an SDK or host implementation can replay to check it produces, or accepts,
the same bytes the contract describes. The protocol itself is described in
the [protocol guide](https://nginxui.com/plugin/protocol).

## Shape

```json
{
  "description": "One line summary of what this vector checks.",
  "method": "plugin.initialize",
  "direction": "host_to_plugin",
  "kind": "request",
  "request": { "...": "the exact frame the caller sends, as a JSON value (one NDJSON line)" },
  "response": { "...": "the exact frame the callee replies with, or omitted for a notification" }
}
```

| Field | Meaning |
| --- | --- |
| `description` | What this vector demonstrates, in one sentence. |
| `method` | The JSON-RPC method under test. |
| `direction` | `host_to_plugin` or `plugin_to_host` — who originates `request`. |
| `kind` | `request` (expects `response`) or `notification` (no `response` key at all). |
| `request` | The exact frame sent, as parsed JSON. Serializing it and appending `\n` reproduces the wire bytes. |
| `response` | Present only for `kind: "request"`: the exact frame the callee replies with. Field order does not matter; a vector's response is compared by value, not by byte-for-byte JSON text. |
| `malformed_params` | Set on a vector whose `params` must not decode into the rpc's request message, so the callee answers `-32602`. |
| `request_raw` | Used instead of `request` only for the two frames that are not valid JSON-RPC 2.0 objects at all (`17-error-parse-error.json`, `18-error-invalid-request.json`): the literal line sent, as a string. |

## Checks

`make check` runs the tests of `tools/methods`, which assert for every vector
that `method` is an rpc of the proto contract with the same `direction` and
`kind` (except the `-32601` vectors, whose method must be unknown or a
streaming rpc that stdio refuses), and that `params` and `result` decode
strictly into the rpc's request and response messages and re-encode to the
same values. A vector marked `malformed_params` must fail to decode; a
capability vector that expects `-32602` for params that decode, such as an
unknown MCP tool, must decode like any other.

## Numbering

Files are numbered in the order a reader would want to see them (handshake
first, then the rest of the lifecycle, then `dns01`, then `host.*`, then the
generic error codes), not by any meaning in the number itself. Adding a
vector does not require renumbering the ones after it — append with the next
free number instead.

## Using these vectors

* **An SDK author** can assert that their client library serializes
  `request` to exactly the given JSON (modulo key order and whitespace) and
  that, given `response`'s bytes on the wire, their client surfaces the same
  result or error their SDK exposes.
* **A plugin author** can feed `request` (serialized plus a trailing `\n`)
  to their plugin's stdin and assert their plugin's stdout produces
  `response`.
* **A host author** can do the reverse: feed `request` to a plugin process
  they control and assert they observe an equivalent frame, or use these as
  fixtures for a mock plugin in their own test suite.

None of these vectors depend on a live nginx-ui host or a live network
service; every value in them is fabricated for the example (fake tokens,
fake domains, fake ids) and safe to commit and reuse.
