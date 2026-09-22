# RFC 0004: The `mcp` capability

| | |
| --- | --- |
| Status | Accepted, implemented |
| Date | 2026-09-23 |
| Spec changes | `spec/14-capabilities-mcp.md` (MCP-1 through MCP-8), MAN-19, MAN-23, MAN-33, SEC-3 (`mcp` permission), SEC-13, NAME-11, CONF-10, WIRE-6 (`-32602` row), WIRE-9 table, `proto/nginxui/plugin/v1/mcp.proto`, `ManifestMCP` in `manifest.proto`, `schema/plugin.schema.json`, vectors 30 through 32, `tools/methods` (`-32602` decode rule scoped to WIRE-6) |
| Reference host | nginx-ui `internal/plugin/capability/mcp.go`, `internal/mcp/server.go` (`AddServerTools`, `DeleteServerTools`), `internal/plugin/manager_capability.go` (`MCPToolName`), `app/src/views/system/plugins/permissions.ts` |
| Reference SDK | nginx-ui-plugin-sdk-go `mcp.go` (`MCPHandler`, `MCPTools`, `MCPText`, `MCPError`, `UnknownTool`) |

## Summary

A plugin may contribute tools to the host's Model Context Protocol server. It
declares them in an `mcp` block, each with a name, a description and a JSON
Schema of its arguments, and requests the new `mcp` permission. While the
plugin is enabled and the permission granted, the host publishes every tool
as `<plugin id with dots replaced by underscores>__<name>`, authorizes calls
as it does for its own tools, and forwards them to `mcp.call`.

## Motivation

nginx-ui already serves MCP tools for nginx control and configuration, and AI
assistants use them to operate a server. What they cannot reach is everything
around it: purging a CDN, rotating a WAF rule, reading a vendor dashboard.
Those integrations belong in plugins, next to the credentials and client
libraries a plugin already carries, and the MCP server is a registry a plugin
can join without the host knowing any vendor.

## Design

### Manifest

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
          "properties": { "zone": { "type": "string" } },
          "required": ["zone"]
        }
      }
    ]
  }
}
```

`input_schema` is a free-form JSON object (`google.protobuf.Struct`) whose
`type` must be `object`; the host publishes it unchanged.

### Method

| Method | Params | Result |
| --- | --- | --- |
| `mcp.call` | `{ tool, arguments }` | `{ content: [{ type: "text", text }], is_error }` |

A tool that ran and failed answers a result with `is_error: true`, exactly as
MCP separates tool errors from protocol errors. An unknown tool is `-32602`,
the code MCP itself uses for it; WIRE-6 now allows `-32602` for params that
decode but name something a capability does not know, and the vector test
keeps requiring undecodable params only of WIRE-6 vectors.

### Naming

The published name is the plugin id with `.` replaced by `_`, then `__`, then
the tool name (NAME-11): `io.github.example.cdn` and `purge_cache` become
`io_github_example_cdn__purge_cache`. Plugin ids never contain `_` (NAME-1),
so the first `__` always ends the prefix and the mapping is reversible and
collision free, whatever the tool name contains. Dots are avoided because
several MCP clients and model APIs only accept `[A-Za-z0-9_-]` in tool
names. A tool name is limited to 48 characters, so the published name stays
within the 128 characters MCP allows.

### Registration

mcp-go, the MCP library of the reference host, can add and delete tools on a
running server and notifies connected clients with
`notifications/tools/list_changed`. The reference host therefore publishes
real per-tool entries instead of a dispatching proxy tool:

* `RegisterMCP` publishes the tools of the enabled plugins at boot and
  subscribes to the `plugin.changed` event, which install, upgrade,
  uninstall, enable, disable and cluster sync all raise. Each event triggers a
  reconcile that withdraws tools whose plugin is gone, disabled or waiting for
  a permission approval, republishes tools whose declaration changed, and adds
  new ones.
* A tool is published only while the plugin is enabled, its permissions are
  approved and they include `mcp` (MCP-7).
* A tool whose schema carries header annotations the MCP library rejects is
  skipped with a warning instead of crashing the server.
* A call starts an `on_demand` plugin, waits up to 60 seconds, and turns a
  JSON-RPC error, a timeout or an unavailable plugin into a tool error result.
  A race between a disable and a call therefore ends in a tool error, never a
  hang.

## Compatibility

Additive. The permission is new, so upgrading a plugin to a version that
starts declaring `mcp` goes through the permission re-approval of SEC-9 and
its tools appear only once a person approved it.

## Alternatives considered

* **One proxy tool per plugin** (`<plugin>__call` with the tool name as an
  argument). Needed only for a library that cannot remove tools; it hides the
  tool list and every schema from the assistant, which is what makes MCP
  tools useful.
* **Unprefixed names with collision detection.** Two plugins, or a plugin and
  a later built-in tool, could claim the same name, and the winner would
  depend on install order.
* **Declaring tools at runtime through a `host.*` call.** It would let a
  plugin change its tool surface without a manifest change, and so without the
  person ever seeing it.

## Security considerations

An MCP tool lets an AI assistant run plugin code with the plugin's
credentials. Three things bound that: the `mcp` permission, shown in the
install and approval dialogs as "Offer its tools to AI assistants connected
to Nginx UI through MCP, which can then run them"; the host's own MCP
authorization, where every plugin tool needs the write scope of a service
token or a secure session, because the reference host classifies unknown
tools as mutating; and MCP-5, which makes a plugin validate arguments as
untrusted input. Tool results must not contain credentials (SEC-7).

## Future work

* A `read_only` hint in the tool declaration, so read-only plugin tools can be
  called with the read scope.
* Content types beyond `text` (images, resources) once a plugin needs them.
