# 20. Capability: `log.sink`

A plugin declaring `"log.sink"` in `capabilities` receives the nginx access
log lines of the host while nginx writes them, parsed into fields, and ships
them wherever it likes: a log store, a SIEM, a metrics pipeline. The host
reads the logs, batches the lines and streams every batch to the plugin. The
one method here is a host → plugin client stream that exists on the gRPC
transport only (`spec/03-wire-protocol.md` WIRE-12): an access log produces
far more lines than a request per line on stdio could carry.

| Method | Required | Meaning |
| --- | --- | --- |
| `log.push` | yes | Receive one batch of access log entries, one message per entry, and answer once. gRPC only. |

Access logs hold client addresses and every URL requested, so a `log.sink`
plugin MUST request the `log.read` permission (SEC-16).

## Manifest block

## LOGSINK-1

`permissions` MUST include `"log.read"` whenever `capabilities` includes
`"log.sink"` (MAN-38). The `log_sink` block is optional; every field has a
default:

```json
{
  "capabilities": ["log.sink"],
  "permissions": ["log.read", "network"],
  "log_sink": {
    "batch_size": 512,
    "flush_interval_ms": 1000,
    "formats": ["combined"]
  }
}
```

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `batch_size` | integer | no | Most entries the host sends in one `log.push` stream. `0` or absent means `256`. |
| `flush_interval_ms` | integer | no | Longest time the host keeps a stream open once its first entry was sent, in milliseconds. `0` or absent means `500`. |
| `formats` | string[] | no | The formats of the lines the plugin wants (LOGSINK-3). Empty or absent means every line. |

A `log_sink` block in a manifest that does not declare the capability has no
effect.

## LOGSINK-2

`batch_size` MUST be between `0` and `4096`. `flush_interval_ms` MUST be `0`
or at least `50`. Together they bound the latency and the size of a batch: a
stream is closed as soon as it carries `batch_size` entries or
`flush_interval_ms` passed since its first entry, whichever comes first
(LOGSINK-10).

## LOGSINK-3

Every entry of `formats` MUST be one of the format names below and MUST NOT
appear twice. The names are the values of `LogEntry.format` (LOGSINK-6):

| Format | Lines |
| --- | --- |
| `combined` | Lines the host parsed as the nginx `combined` log format, optionally followed by `$request_time` and `$upstream_response_time`. The parsed fields are set. |
| `raw` | Every other line: a custom `log_format`, a JSON log, a truncated line. Only `raw` and `timestamp` are set, `timestamp` being the time the host read the line. |

A host MUST deliver only the lines whose format the list names, and every
line when the list is empty. A format this spec adds later is a new name; a
plugin that lists formats never receives lines of a format it did not list.

## Transport

## LOGSINK-4

A `log.sink` plugin MUST list `grpc` in the `transports` of its
`plugin.initialize` result and MUST serve the rpc
`/nginxui.plugin.v1.LogSink/Push` over gRPC (WIRE-11). `log.push` has no
JSON-RPC form: a host MUST NOT send it on stdio, and a plugin that receives
it on stdio MUST answer `-32601` like for any unknown method (WIRE-12,
vector 47). A host that runs a `log.sink` plugin which does not list `grpc`,
or whose gRPC channel cannot be used, delivers nothing to it and reports why
(LOGSINK-11).

## `log.push`

## LOGSINK-5

The host opens a client stream, sends between one and `batch_size`
`LogSinkPushRequest` messages, one per access log line, and half-closes the
stream. The plugin reads until the end of the stream and then answers with
one `LogSinkPushResponse`, or fails the call with a gRPC status (WIRE-11).
Shown in the protobuf JSON mapping (WIRE-10), one request message is:

```json
{
  "log_path": "/var/log/nginx/access.log",
  "entry": {
    "timestamp": "2026-09-23T08:15:02Z",
    "remote_addr": "203.0.113.7",
    "request_method": "GET",
    "request_uri": "/index.html?lang=en",
    "protocol": "HTTP/1.1",
    "status": 200,
    "body_bytes_sent": 612,
    "referer": "https://example.com/",
    "user_agent": "Mozilla/5.0 (X11; Linux x86_64)",
    "request_time": 0.004,
    "upstream_response_time": 0.003,
    "raw": "203.0.113.7 - - [23/Sep/2026:08:15:02 +0000] \"GET /index.html?lang=en HTTP/1.1\" 200 612 \"https://example.com/\" \"Mozilla/5.0 (X11; Linux x86_64)\" 0.004 0.003",
    "format": "combined"
  }
}
```

and the answer is:

```json
{ "accepted": 3, "rejected": 0 }
```

| Field | Type | Meaning |
| --- | --- | --- |
| `log_path` | string | Absolute path of the access log the line was read from. |
| `entry` | object | The line, see LOGSINK-6. |
| `accepted` | integer | Entries of the stream the plugin kept. |
| `rejected` | integer | Entries of the stream the plugin received and discarded. |

`accepted` plus `rejected` SHOULD equal the number of entries the stream
carried. A host counts the entries it sent minus `accepted` as rejected. A
stream without any message is valid and answers `{}`.

## LOGSINK-6

`LogEntry` carries the fields the host extracted from one line. A field the
host could not extract is absent (WIRE-10: absent and default mean the same).

| Field | Type | nginx variable | Meaning |
| --- | --- | --- | --- |
| `timestamp` | string | `$time_local` | Time of the request in RFC 3339 form. For a `raw` line, the time the host read it. The reference host writes UTC. |
| `remote_addr` | string | `$remote_addr` | Client address. |
| `request_method` | string | from `$request` | Request method, e.g. `GET`. |
| `request_uri` | string | from `$request` | Request target as the client sent it, with the query string. |
| `protocol` | string | from `$request` | Protocol, e.g. `HTTP/1.1`. |
| `status` | integer | `$status` | Response status. |
| `body_bytes_sent` | number | `$body_bytes_sent` | Response body size in bytes, a `double` (WIRE-10). |
| `referer` | string | `$http_referer` | Referer header. |
| `user_agent` | string | `$http_user_agent` | User agent header. |
| `upstream_addr` | string | `$upstream_addr` | Upstream servers that handled the request. |
| `request_time` | number | `$request_time` | Processing time in seconds. |
| `upstream_response_time` | number | `$upstream_response_time` | Upstream time in seconds. |
| `host` | string | `$host` | Host the request was served for. |
| `raw` | string | — | The line as nginx wrote it, without the line break. Always set. |
| `format` | string | — | `combined` or `raw` (LOGSINK-3). |

The fields a host extracts depend on the log formats it can parse. The
reference host parses the `combined` format and its extension by
`$request_time` and `$upstream_response_time`, which carry neither
`$upstream_addr` nor `$host`; a plugin that needs those parses `raw`.

## LOGSINK-7

A plugin SHOULD answer a stream promptly: the host does not open the next
stream of the plugin before the answer and drops lines while it waits
(LOGSINK-9). A plugin that ships the entries elsewhere SHOULD buffer them
and answer before the upstream acknowledged them. An error status means the
plugin lost the whole batch; the host does not send it again. A plugin MUST
treat every field as untrusted input, since clients choose the request
target, the referer and the user agent, and MUST treat the entries as
personal data (SEC-16).

## Host behavior

## LOGSINK-8

A host MUST deliver to every enabled `log.sink` plugin that was granted
`log.read` (SEC-4) the lines appended to the access logs it knows while the
plugin is enabled, from logs a person allowed it to read (the reference host
applies the log directory whitelist of its log viewer), filtered by
`formats` (LOGSINK-3). A host MUST NOT replay lines written before the
plugin was enabled or before the host started, and MUST NOT deliver a line
twice when a log is rotated: it starts a new file at its first line and
follows a file from its end otherwise. A host MAY skip compressed or
unreadable logs. A host SHOULD read the logs only while at least one such
plugin is enabled.

## LOGSINK-9

A host MUST NOT let a slow or stopped plugin hold back nginx, its log
pipeline or other plugins. It MUST queue the lines of every plugin in a
bounded buffer, MUST drop a line that does not fit and MUST count what it
dropped. The reference host queues 8192 entries per plugin.

## LOGSINK-10

A host MUST close a stream as soon as it carries `batch_size` entries or
`flush_interval_ms` passed since its first entry, whichever comes first,
and SHOULD open the next stream as soon as entries are waiting. It MUST
bound the whole stream with a deadline (30 seconds in the reference host).
When a stream fails, the host counts its entries as dropped and waits before
the next stream, doubling the wait after every failure in a row; the
reference host waits 1 second at first and 30 seconds at most. An
`on_demand` plugin (LIFE-13) is started for a stream and may go idle
between streams.

## LOGSINK-11

A host MUST stop delivering to a plugin, and MUST discard its queue, once the
plugin is disabled, uninstalled or upgraded to a version whose permissions
still need approval. When a running `log.sink` plugin does not list `grpc`, a
host MUST report it as the plugin's last error and deliver nothing; the
reference host reports `log.sink requires the grpc transport`. A host MUST
show, for every `log.sink` plugin, how many entries the plugin accepted, how
many it rejected and how many the host dropped; the reference host reports
them as `streamed_log_entries`, `rejected_log_entries` and
`dropped_log_entries` in the plugin info.
