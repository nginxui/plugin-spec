# RFC 0010: The `log.sink` capability and streaming rpcs

| | |
| --- | --- |
| Status | Accepted, implemented |
| Date | 2026-09-23 |
| Spec changes | `spec/20-capabilities-logsink.md` (LOGSINK-1 through LOGSINK-11), WIRE-12, the `streaming` method option, WIRE-9 and WIRE-11 tables, LIFE-2 (`transports`), LIFE-10 (shutdown list), MAN-19, MAN-23, MAN-38, SEC-3 (`log.read` row), SEC-16, CONF-1, CONF-4, CONF-5, CONF-7, CONF-16, NAME intro, `proto/nginxui/plugin/v1/log.proto`, `options.proto`, `ManifestLogSink` in `manifest.proto`, `tools/methods` (`streaming`), `schema/plugin.schema.json`, vector 47 |
| Reference host | nginx-ui `internal/nginx_log/sink` (hub and access log tail), `internal/plugin/logsink.go` (per plugin queue and streamer), `internal/plugin/capability/logsink.go`, `internal/plugin/grpcbridge` (client streams), `internal/plugin/conformance.go` (LOGSINK-4, LOGSINK-5), `app/src/views/system/plugins/PluginDrawer.vue` |
| Reference SDK | plugin-sdk-go `logsink.go` (`LogSinkHandler`), `grpc.go` (client streams) |

## Summary

A plugin may receive the nginx access log lines of the host while they are
written. It declares `log.sink`, requests `log.read`, and may tune the
batches in an optional `log_sink` block. The host tails the access logs,
parses every new line and streams batches to the plugin over a client
streaming gRPC rpc, `log.push`, which has no stdio form. Every plugin gets a
bounded queue of its own, so a slow plugin loses lines instead of slowing
anything down, and the host reports how many lines each plugin accepted,
rejected and lost.

## Motivation

Shipping access logs to a log store, a SIEM or a metrics pipeline is a
common request, and every destination has its own protocol, so the core
cannot ship them. The capability is also the first high volume flow of the
contract: a busy server writes thousands of lines per second. A JSON-RPC
request per line on the stdio pipe would cost a round trip, a JSON encoding
and a pipe write per line and share the pipe with the liveness pings. The
optional gRPC transport of WIRE-11 already exists; a client stream on it
carries one protobuf message per line and one answer per batch.

## Design

### Contract

```proto
service LogSink {
  rpc Push(stream LogSinkPushRequest) returns (LogSinkPushResponse) {
    option (nginxui.plugin.v1.rpc_name) = "log.push";
    option (nginxui.plugin.v1.streaming) = true;
  }
}
message LogSinkPushRequest { string log_path = 1; LogEntry entry = 2; }
message LogSinkPushResponse { uint32 accepted = 1; uint32 rejected = 2; }
```

`LogEntry` carries `timestamp` (RFC 3339), `remote_addr`,
`request_method`, `request_uri`, `protocol`, `status`, `body_bytes_sent`
(a double, WIRE-10), `referer`, `user_agent`, `upstream_addr`,
`request_time`, `upstream_response_time`, `host`, `raw` and `format`.

The new method option `streaming` marks the rpc. `tools/methods` requires it
exactly on client streaming rpcs, rejects server and bidirectional streams
and writes `"streaming": true` to `spec/methods.json`. The bridges of the
reference host and the Go SDK read the flag from the descriptors, keep the
rpc out of unary dispatch and never route it to stdio, where the name has
no handler and answers `-32601`.

### Manifest

```json
{
  "capabilities": ["log.sink"],
  "permissions": ["log.read"],
  "log_sink": { "batch_size": 256, "flush_interval_ms": 500, "formats": ["combined"] }
}
```

`batch_size` defaults to 256 and is at most 4096, `flush_interval_ms`
defaults to 500 and is at least 50. `formats` picks `combined` lines (parsed
fields set) or `raw` lines (lines the host could not parse); empty means
both.

### Reference host

* **Where the lines come from.** The host parses access log lines in one
  place only, the incremental indexer (`internal/cron/incremental_indexing.go`),
  and it is not a fit: it runs every 15 minutes by default and only while
  advanced indexing is on, and it re-reads rotated and compressed files from
  the start, which would deliver lines twice. So `internal/nginx_log/sink`
  adds its own feeder, a single goroutine that runs only while at least one
  plugin is subscribed. It polls the access logs the host knows (the paths
  of the log list, filtered by the log viewer's whitelist) every second,
  starts at the end of a file it sees for the first time, reads a rotated
  file from its first line, keeps partial lines until their line break,
  reads at most 4 MiB per file and poll, and parses with the indexer's
  parser without geo and user agent enrichment. With no subscriber there is
  no goroutine, no open file and no work.
* **Fan-out.** The hub hands every batch of lines to every subscriber.
  `internal/plugin/capability.RegisterLogSink` subscribes one queue per
  enabled `log.sink` plugin that was granted `log.read`; the queue filters by
  `formats`, holds 8192 entries and counts what it drops.
* **Streamer.** One goroutine per plugin waits for entries, acquires the
  plugin (starting an `on_demand` one), opens a stream over the plugin's gRPC
  channel, sends up to `batch_size` entries encoded with the generated
  message types, closes the stream after `batch_size` entries or
  `flush_interval_ms`, and adds `accepted` and `rejected` to the counters.
  A failed stream counts its entries as dropped and backs off from 1 to 30
  seconds. A plugin that does not advertise `grpc` gets the last error `log.sink
  requires the grpc transport` and nothing is sent.
* **Reporting.** The plugin info carries `streamed_log_entries`,
  `rejected_log_entries` and `dropped_log_entries`, shown in the overview
  of the plugin drawer.

### Reference SDK

`sdk.Plugin.LogSink` takes a `LogSinkHandler`:

```go
type LogSinkHandler interface {
	Push(ctx context.Context, batch []sdk.LogEntry) (accepted int, err error)
}
```

The catch-all gRPC handler of the SDK detects the streaming rpc from the
descriptors, reads the stream until its end and calls `Push` with the batch,
in chunks of at most 4096 entries. Setting `LogSink` adds `log.sink` to the
capabilities and keeps the gRPC transport on even under `WithoutGRPC` or
`NGINX_UI_PLUGIN_DISABLE_GRPC`, since the capability cannot work without it.

## Compatibility

A new optional capability, permission, block, service and method option.
Nothing existing changes meaning (VER-1). A host that does not know
`log.sink` rejects the manifest (MAN-19) instead of running a plugin that
would wait for lines forever. `spec/methods.json` gains a member that is
omitted for every unary rpc.

## Alternatives considered

* **One JSON-RPC notification per line on stdio.** Simple, but the stdio
  pipe carries the liveness pings and host API calls too, and a notification
  per line costs far more than a protobuf message on a stream.
* **Unary gRPC calls with a repeated field.** Works, but a batch then lives
  in memory twice on both sides and a large batch hits the message size
  limit; a stream sends the lines as they come and bounds nothing but the
  count.
* **A server stream the plugin opens.** Inverts the direction of every other
  capability and needs the plugin to reconnect; the host would still need
  its queue per plugin.
* **Feeding from the incremental indexer.** See above: too late, gated by
  a setting, and duplicating after rotation.

## Security considerations

Access logs are personal data and may carry secrets in query strings, so the
lines only go to a plugin a person granted `log.read` (SEC-16), only from
logs the host's own log viewer may read, and never over stdio or into the
host's log. Lines are attacker controlled, so a plugin treats every field as
untrusted input.

## Future work

* Parsing more log formats (JSON logs, `$upstream_addr` and `$host`).
* Error logs as a second stream.
* Delivering the lines of remote nodes of a cluster.
