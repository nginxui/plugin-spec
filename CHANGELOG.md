# Changelog

Changes to the plugin contract. The developer guide at
[nginxui.com/plugin](https://nginxui.com/plugin/overview) describes the
behavior behind each entry.

## Unreleased

First public version, plugin API version 1 (`api_version = 1`):

- Proto contract for the lifecycle, the host API, events, and the `dns01`,
  `http`, `notify`, `probe`, `mcp`, `storage`, `cert.deploy`,
  `security.blocklist`, `upstream.discovery` and `log.sink` capabilities,
  over JSON-RPC 2.0 on stdio and over gRPC.
- JSON Schemas of `plugin.json`, the marketplace catalog and the partner
  keyring.
- Test vectors of the wire protocol and a Python example plugin.
- A catalog release can carry its `notes` in Markdown next to
  `release_notes_url`.
