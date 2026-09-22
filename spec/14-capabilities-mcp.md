# 14. Capability: `mcp`

A plugin declaring `"mcp"` in `capabilities` contributes tools to the host's
[Model Context Protocol](https://modelcontextprotocol.io) server, so an AI
assistant connected to the host can call them next to the host's built-in
tools. The host publishes each declared tool under a name derived from the
plugin id, authorizes every call as it does for its own tools, and forwards
it to the plugin. The one method here is a host → plugin request.

| Method | Required | Meaning |
| --- | --- | --- |
| `mcp.call` | yes | Run one tool. |

## Manifest block

## MCP-1

The manifest MUST include an `mcp` block with at least one entry in
`mcp.tools`, and MUST request the `mcp` permission, whenever `capabilities`
includes `"mcp"` (MAN-33):

```json
{
  "capabilities": ["mcp"],
  "permissions": ["mcp", "network"],
  "mcp": {
    "tools": [
      {
        "name": "purge_cache",
        "description": "Purge cached paths of a CDN zone.",
        "input_schema": {
          "type": "object",
          "properties": {
            "zone": { "type": "string", "description": "Zone name, e.g. example.com" },
            "paths": { "type": "array", "items": { "type": "string" } }
          },
          "required": ["zone"]
        }
      }
    ]
  }
}
```

Each entry of `mcp.tools`:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `name` | string | yes | Tool name within the plugin. See MCP-2 and NAME-11. |
| `description` | string | yes | What the tool does, for the AI assistant choosing it. |
| `input_schema` | object | no | JSON Schema of the arguments. See MCP-3. |

## MCP-2

`name` MUST match `^[a-z0-9][a-z0-9_-]{0,47}$` and MUST be unique among the
tools one manifest declares. `description` MUST be a non-empty string.

## MCP-3

`input_schema`, when present, MUST be a JSON Schema object whose `type` is
`"object"`, as the Model Context Protocol requires of a tool's input schema.
An absent `input_schema` means a tool without arguments, published as
`{ "type": "object" }`. A host MUST publish the schema unchanged.

## `mcp.call`

## MCP-4

Request:

```json
{
  "jsonrpc": "2.0", "id": 37, "method": "mcp.call",
  "params": { "tool": "purge_cache", "arguments": { "zone": "example.com", "paths": ["/index.html"] } }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `tool` | string | The `name` of the tool as the manifest declares it, without the host prefix. |
| `arguments` | object | The arguments the MCP client sent. Absent or `{}` for a tool without arguments. |

## MCP-5

A plugin MUST treat `arguments` as untrusted input: they come from an AI
assistant, not from a person filling in a form, and MAY violate
`input_schema`. A plugin MUST validate them before acting on them.

## MCP-6

Reply:

```json
{ "jsonrpc": "2.0", "id": 37, "result": { "content": [{ "type": "text", "text": "Purged 1 path in zone example.com." }] } }
```

| Field | Type | Meaning |
| --- | --- | --- |
| `content` | object[] | Content blocks returned to the MCP client, in order. |
| `content[].type` | string | `text`. It is the only type this spec defines. |
| `content[].text` | string | The text of a `text` block. |
| `is_error` | boolean | The tool ran and failed; `content` explains why. |

A tool that ran and failed (the vendor rejected the request, the zone does
not exist) MUST reply with a result whose `is_error` is `true`, so the AI
assistant sees the explanation and can correct itself. A plugin MUST reply
with a JSON-RPC error only when the call could not be handled: `-32602` for
a `tool` the manifest does not declare or for `arguments` that are not an
object, as the Model Context Protocol does for an unknown tool, `-32000` for
anything else. `content` MUST NOT contain a credential (SEC-7). A host MUST
ignore a content block whose `type` it does not know.

## Host behavior

## MCP-7

A host MUST publish a plugin's tools only while the plugin is enabled and the
`mcp` permission is granted to it (SEC-4, SEC-13), and MUST withdraw them
when the plugin is disabled, uninstalled, or upgraded to a version whose
permissions still need approval. It publishes every tool under the name
NAME-11 defines. The reference host adds and removes the tools on its live
MCP server and lets connected clients know through the Model Context
Protocol's `notifications/tools/list_changed`; a host whose MCP server
cannot remove a tool MUST instead answer calls to a withdrawn tool with a
tool error.

## MCP-8

A host MUST authorize a call to a plugin tool before it forwards it, as it
authorizes calls to its own tools, and SHOULD treat every plugin tool as a
tool that changes state; the reference host requires the write scope of an
MCP service token, or a secure session for a signed-in person. A host starts
an `on_demand` plugin to serve the call, reports a JSON-RPC error of the
plugin, a timeout or an unavailable plugin to the MCP client as a tool
result whose `isError` is `true`, and waits 60 seconds for `mcp.call` in the
reference host, which is a recommendation.
