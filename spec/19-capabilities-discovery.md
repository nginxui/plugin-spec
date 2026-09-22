# 19. Capability: `upstream.discovery`

A plugin declaring `"upstream.discovery"` in `capabilities` resolves a
service into the servers that back it right now, through a service registry,
a container orchestrator or a cloud API. It declares the providers it can
resolve through; a person binds an nginx upstream to a service of a
provider, and the host calls the plugin on the refresh interval of the
binding, writes the answer as an `upstream` block in a file of its own and
reloads nginx when the servers changed. The one method here is a host →
plugin request.

| Method | Required | Meaning |
| --- | --- | --- |
| `discovery.resolve` | yes | Resolve one service into its current servers. |

An `upstream.discovery` plugin talks to the provider, so it MUST request the
`network` permission (SEC-3).

## Manifest block

## DISCOVERY-1

The manifest MUST include a `discovery` block with at least one entry in
`discovery.providers`, and MUST request the `network` permission, whenever
`capabilities` includes `"upstream.discovery"` (MAN-37):

```json
{
  "capabilities": ["upstream.discovery"],
  "permissions": ["network"],
  "discovery": {
    "providers": [
      {
        "code": "registry",
        "name": "Service registry",
        "configuration": {
          "fields": [
            { "key": "address", "display_name": "Registry address", "required": true },
            { "key": "token", "display_name": "ACL token", "secret": true },
            { "key": "healthy_only", "type": "bool", "display_name": "Only healthy instances" }
          ]
        }
      }
    ]
  }
}
```

Each entry of `discovery.providers`:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `code` | string | yes | Identifier of the provider on the wire, sent as `provider`. See NAME-10. |
| `name` | string | yes | Display name of the provider. |
| `configuration.fields` | object[] | no | The fields of the provider form, as in NOTIFY-4. |

## DISCOVERY-2

`code` MUST match `^[a-z0-9-]{2,32}$` and MUST be unique among the providers
one manifest declares. See `spec/11-naming.md` NAME-10 for cross-plugin
uniqueness.

## DISCOVERY-3

`name` MUST be a non-empty string. `configuration.fields` follows NOTIFY-4.

## `discovery.resolve`

## DISCOVERY-4

Request:

```json
{
  "jsonrpc": "2.0", "id": 50, "method": "discovery.resolve",
  "params": {
    "provider": "registry",
    "config": { "address": "https://registry.example.com:8500", "token": "tok_live_xxx" },
    "service": "api"
  }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `provider` | string | The `code` of the provider, without any host prefix. |
| `config` | map\<string, string\> | The field values the person filled in, keyed by `key` (NOTIFY-4). |
| `service` | string | The service to resolve, in the terms of the provider (a service name, a label selector, a tag). |

## DISCOVERY-5

On success a plugin MUST reply with every server that backs the service
right now:

```json
{
  "jsonrpc": "2.0", "id": 50,
  "result": {
    "targets": [
      { "address": "10.0.1.12", "port": 8080, "weight": 1, "tags": ["zone-a"] },
      { "address": "10.0.2.7", "port": 8080, "weight": 2, "tags": ["zone-b", "canary"] },
      { "address": "2001:db8::17", "port": 8443 }
    ],
    "ttl_seconds": 30
  }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `targets` | object[] | Every server of the service. An empty list means the service has no server right now. |
| `targets[].address` | string | An IPv4 or IPv6 address, without brackets, or a host name. No port, no scheme. |
| `targets[].port` | integer | TCP port, `1` to `65535`. |
| `targets[].weight` | integer | Relative weight, `1` to `1000`. `0` or absent means `1`. |
| `targets[].tags` | string[] | Labels the provider attaches to the server, for display. |
| `ttl_seconds` | integer | How long the answer stays fresh. `0` or absent leaves the refresh interval to the host (DISCOVERY-11). |

A plugin SHOULD list every server once.

## DISCOVERY-6

A plugin MUST reply with an error when it could not resolve the service, so
the host keeps the servers it has. It MUST reply `-32003` (`data.field`
naming the field at fault) for anything caused by what the person entered: a
`config` that leaves a required field empty, rejected credentials, or a
`service` the provider does not know, which names `service`. It MUST reply
`-32000` for anything else (the provider is down, a network failure).

## DISCOVERY-7

A plugin MUST treat `config` values as secret, as NOTIFY-7 describes.

## Host behavior

## DISCOVERY-8

A host MUST offer each provider of every enabled `upstream.discovery` plugin
and MUST stop calling `discovery.resolve` for it once the plugin is disabled,
uninstalled or upgraded to a version whose permissions still need approval.
A host MUST keep provider codes apart from anything of its own; the
reference host stores a binding's kind as `plugin:<code>`. A host MUST render
the provider form from `configuration.fields` as NOTIFY-10 describes, MUST
NOT store a configuration that leaves a `required` field empty, MUST store
the values encrypted at rest, and MUST call only the plugin that owns the
provider `code` at the time of the call (NAME-10), starting an `on_demand`
plugin to do so.

## DISCOVERY-9

A host MUST treat the answer as untrusted input. It MUST write a target to
the nginx configuration only when `address` parses as an IPv4 or IPv6
address or is a valid host name (dot separated labels of letters, digits,
hyphens and underscores, at most 253 characters), `port` is between 1 and 65535 and
`weight` between 0 and 1000, MUST drop every other target and MUST report how
many it dropped. A host MUST NOT copy any other text of the answer, `tags`
included, into the configuration. When no valid target is left, the host
MUST keep the file it has, since an `upstream` block without a server is
invalid, and records the resolution as failed. The reference host removes
duplicate targets and sorts the rest, so the same answer always produces the
same file.

## DISCOVERY-10

A host MUST write the servers of each binding to a file of its own inside the
nginx configuration directory, replacing it atomically, and MUST reload
nginx only when the content of the file changed. Before the reload it MUST
test the configuration and, when the test fails, MUST put the previous file
back and report the failure instead of reloading. A host MUST NOT add the
file to the configuration itself: it shows the person the `include` line to
add at the `http` level. The name of the upstream is chosen by the person
and MUST be checked against `^[A-Za-z0-9_][A-Za-z0-9_.-]{0,63}$` and against
the other bindings. The reference host writes
`<nginx conf dir>/upstreams/<name>.conf`:

```nginx
# generated by nginx-ui
upstream api {
    server 10.0.1.12:8080 weight=1;
    server 10.0.2.7:8080 weight=2;
    server [2001:db8::17]:8443 weight=1;
    keepalive 32;
}
```

where the last lines are the extra directives the person entered for the
binding (`keepalive`, `least_conn`, a `backup` server), which MUST NOT
contain `{` or `}`, so they cannot leave the block. It shows `include upstreams/<name>.conf;`, a
path nginx resolves against the directory of `nginx.conf`, to be placed in
the `http` block or at the top level of a site file. A binding that is
disabled keeps its file as it is; deleting a binding removes its file, and
the reference host refuses the deletion while the configuration still
includes the file.

## DISCOVERY-11

A host MUST resolve every enabled binding again after its refresh interval,
and SHOULD resolve it earlier when the last answer carried a shorter
`ttl_seconds`. A person MUST be able to set the interval of a binding and to
ask for a refresh at any time. A host MUST record the outcome of the last
resolution of each binding (time, success or failure, the error or the
number of servers written and dropped) and MUST show it next to the binding.
The reference host uses 60 seconds as the default interval and 10 seconds as
the shortest one, retries a failed resolution after the interval of the
binding or after 5 minutes, whichever comes first, and waits 30 seconds for
`discovery.resolve`; these values are a recommendation.
