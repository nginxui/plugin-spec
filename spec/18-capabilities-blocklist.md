# 18. Capability: `security.blocklist`

A plugin declaring `"security.blocklist"` in `capabilities` fetches lists of
addresses to deny from places that publish them: a threat intelligence feed,
a reputation service, a ban list shared between servers. It declares the
kinds of source it can fetch from; a person configures one or more sources of
a kind, and the host calls the plugin on the refresh interval of each source,
turns the answer into nginx `deny` rules in a file of its own and reloads
nginx when the list changed. The one method here is a host → plugin request.

| Method | Required | Meaning |
| --- | --- | --- |
| `blocklist.fetch` | yes | Fetch the complete current list of one source. |

A `security.blocklist` plugin talks to the source, so it MUST request the
`network` permission (SEC-3).

## Manifest block

## BLOCKLIST-1

The manifest MUST include a `blocklist` block with at least one entry in
`blocklist.sources`, and MUST request the `network` permission, whenever
`capabilities` includes `"security.blocklist"` (MAN-36):

```json
{
  "capabilities": ["security.blocklist"],
  "permissions": ["network"],
  "blocklist": {
    "sources": [
      {
        "code": "threatfeed",
        "name": "ThreatFeed",
        "refresh_seconds": 900,
        "configuration": {
          "fields": [
            { "key": "api_key", "display_name": "API key", "required": true, "secret": true },
            { "key": "min_score", "type": "number", "display_name": "Minimum score", "help_text": "Only list addresses scored at least this high" }
          ]
        }
      }
    ]
  }
}
```

Each entry of `blocklist.sources` describes a kind of source:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `code` | string | yes | Identifier of the source kind on the wire, sent as `source`. See NAME-10. |
| `name` | string | yes | Display name of the source kind. |
| `configuration.fields` | object[] | no | The fields of the source form, as in NOTIFY-4. |
| `refresh_seconds` | integer | no | Default refresh interval of a source of this kind. Absent or `0` means 3600. |

## BLOCKLIST-2

`code` MUST match `^[a-z0-9-]{2,32}$` and MUST be unique among the source
kinds one manifest declares. See `spec/11-naming.md` NAME-10 for
cross-plugin uniqueness.

## BLOCKLIST-3

`name` MUST be a non-empty string. `configuration.fields` follows NOTIFY-4.
`refresh_seconds` MUST be `0` or at least `60`: a list that changes faster
than once a minute cannot be served by reloading nginx.

## `blocklist.fetch`

## BLOCKLIST-4

Request:

```json
{ "jsonrpc": "2.0", "id": 48, "method": "blocklist.fetch", "params": { "source": "threatfeed", "config": { "api_key": "key_live_xxx", "min_score": "80" } } }
```

| Field | Type | Meaning |
| --- | --- | --- |
| `source` | string | The `code` of the source kind, without any host prefix. |
| `config` | map\<string, string\> | The field values the person filled in, keyed by `key` (NOTIFY-4). |

## BLOCKLIST-5

On success a plugin MUST reply with the complete current list of the source,
not with the changes since the last call:

```json
{
  "jsonrpc": "2.0", "id": 48,
  "result": {
    "entries": [
      { "cidr": "198.51.100.23", "reason": "SSH brute force" },
      { "cidr": "203.0.113.0/24", "reason": "Botnet command and control" },
      { "cidr": "2001:db8:bad::/48" }
    ],
    "ttl_seconds": 900
  }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `entries` | object[] | Every address and network to deny. An empty or absent list denies nothing. |
| `entries[].cidr` | string | An IPv4 or IPv6 address, or a network in CIDR notation (`203.0.113.0/24`, `2001:db8::/32`). No port, no zone, no host name. |
| `entries[].reason` | string | Why the source lists the entry, for display. Optional. |
| `ttl_seconds` | integer | How long the list stays fresh. `0` or absent leaves the refresh interval to the host (BLOCKLIST-11). |

A plugin SHOULD send every address once and SHOULD NOT send an entry that
covers every address (`0.0.0.0/0`, `::/0`), which a host drops (BLOCKLIST-9).

## BLOCKLIST-6

A plugin MUST reply with an error, never with an empty list, when it could
not fetch the list: an empty list tells the host to deny nothing, while an
error makes it keep the list it has. It MUST reply `-32003` (`data.field`
naming the field at fault) for anything caused by what the person entered, a
`config` that leaves a required field empty included, and `-32000` for
anything else (the source is down, a network failure).

## BLOCKLIST-7

A plugin MUST treat `config` values as secret, as NOTIFY-7 describes, and
MUST NOT write them to a log line, an error message or `reason`.

## Host behavior

## BLOCKLIST-8

A host MUST offer each source kind of every enabled `security.blocklist`
plugin and MUST stop calling `blocklist.fetch` for it once the plugin is
disabled, uninstalled or upgraded to a version whose permissions still need
approval. A host MUST keep kind codes apart from any source kind of its own;
the reference host stores a source's kind as `plugin:<code>`. A host MUST
render the source form from `configuration.fields` as NOTIFY-10 describes,
MUST NOT store a configuration that leaves a `required` field empty, MUST
store the values encrypted at rest, and MUST call only the plugin that owns
the kind `code` at the time of the call (NAME-10), starting an `on_demand`
plugin to do so.

## BLOCKLIST-9

A host MUST treat the answer as untrusted input. It MUST write an entry to
the nginx configuration only after parsing it as an IPv4 or IPv6 address or
CIDR network, MUST drop every other entry and every entry that covers all
addresses, and MUST report how many it dropped. A host MUST NOT copy any
other text of the answer, `reason` included, into the configuration. The
reference host normalizes every entry (the network address of a prefix, an
IPv4-mapped IPv6 address as IPv4), removes duplicates, sorts the list so the
same answer always produces the same file, and keeps at most 100000 entries.

## BLOCKLIST-10

A host MUST write the list of each source to a file of its own inside the
nginx configuration directory, replacing it atomically, and MUST reload nginx
only when the content of the file changed. Before the reload it MUST test the
configuration and, when the test fails, MUST put the previous file back and
report the failure instead of reloading. When a fetch fails the host MUST
keep the file it has. A host MUST NOT add the file to a server block itself:
it shows the person the `include` line to add where they want the list to
apply. The reference host writes `<nginx conf dir>/blocklists/<id>.conf`:

```nginx
# generated by nginx-ui
deny 198.51.100.23;
deny 203.0.113.0/24;
deny 2001:db8:bad::/48;
```

and shows `include blocklists/<id>.conf;`, a path nginx resolves against the
directory of `nginx.conf`. A source that is disabled keeps its file as it is;
deleting a source removes its file, and the reference host refuses the
deletion while the configuration still includes the file, since nginx would
no longer start.

## BLOCKLIST-11

A host MUST fetch every enabled source again after its refresh interval, and
SHOULD fetch it earlier when the last answer carried a shorter
`ttl_seconds`, never more often than once a minute. A person MUST be able to
set the interval of a source, starting from the `refresh_seconds` of its
kind, and to ask for a refresh at any time. A host MUST record the outcome of
the last fetch of each source (time, success or failure, the error or the
number of entries written and dropped) and MUST show it next to the source.
The reference host retries a failed fetch after 5 minutes, or after the
interval of the source when that is shorter, and waits 60 seconds for
`blocklist.fetch`; these values are a recommendation.
