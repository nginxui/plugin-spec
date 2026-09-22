# 12. Capability: `notify`

A plugin declaring `"notify"` in `capabilities` delivers host notifications
through vendor channels (a chat service, a push gateway, a paging system).
The host offers every channel the plugin declares next to its built-in
notification channels; a person configures a channel once, and the host
calls the plugin whenever a notification is routed to it. Both methods here
are host → plugin requests.

| Method | Required | Meaning |
| --- | --- | --- |
| `notify.send` | yes | Deliver one notification through a channel. |
| `notify.validate` | no | Check a channel configuration without sending anything. |

A plugin that does not implement `notify.validate` MUST reply with `-32002`
(Unsupported, WIRE-6) or `-32601`, and a host MUST treat both as "no
opinion" rather than as a rejected configuration.

The `notify` capability is unrelated to the `notify` permission: the
permission gates `host.notify`, with which a plugin raises a notification in
the host (HOST-12). A `notify` plugin needs no permission to receive
`notify.send`; it SHOULD request `network` (SEC-3), since it talks to a
vendor.

## Manifest block

## NOTIFY-1

The manifest MUST include a `notify` block with at least one entry in
`notify.channels` whenever `capabilities` includes `"notify"` (MAN-31):

```json
{
  "notify": {
    "channels": [
      {
        "code": "mychat",
        "name": "MyChat",
        "configuration": {
          "fields": [
            { "key": "webhook_url", "display_name": "Webhook URL", "required": true },
            { "key": "token", "display_name": "Token", "help_text": "Bot token of the workspace", "required": true, "secret": true },
            { "key": "mention_all", "type": "bool", "display_name": "Mention everyone on errors" }
          ]
        }
      }
    ]
  }
}
```

Each entry of `notify.channels`:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `code` | string | yes | Identifier of the channel on the wire. See NAME-10. |
| `name` | string | yes | Display name of the channel. |
| `configuration.fields` | object[] | no | The fields of the channel form, in display order. |

## NOTIFY-2

`code` MUST match `^[a-z0-9-]{2,32}$` and MUST be unique among the channels
one manifest declares. See `spec/11-naming.md` NAME-10 for cross-plugin
uniqueness.

## NOTIFY-3

`name` MUST be a non-empty string.

## NOTIFY-4

`configuration.fields` drives the form a host renders for the channel. The
same shape, `ConfigurationSchema` in `manifest.proto`, describes the form of
a probe kind (PROBE-3). Each field:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `key` | string | yes | Key of the value in `config`. MUST be non-empty and unique within the channel. |
| `type` | string | no | `text` (the default when absent), `textarea`, `number` or `bool`. |
| `display_name` | string | yes | Field label. MUST be non-empty. |
| `help_text` | string | no | Field description. |
| `required` | boolean | no | The host MUST NOT store a configuration that leaves the field empty. |
| `secret` | boolean | no | The value is a credential: the host renders a masked input and never logs it. |

Values travel as strings whatever the `type`: a `number` field as a decimal
number (`"30"`), a `bool` field as `"true"` or `"false"`. A field a person
left empty MAY be absent from `config`.

## `notify.send`

## NOTIFY-5

Request:

```json
{
  "jsonrpc": "2.0", "id": 30, "method": "notify.send",
  "params": {
    "channel": "mychat",
    "config": { "webhook_url": "https://chat.example/hooks/xxx", "token": "tok_live_xxx" },
    "title": "Certificate Expiring Soon",
    "content": "Certificate example.com expires in 7 days.",
    "severity": "warning"
  }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `channel` | string | The `code` of the channel, without any host prefix. |
| `config` | map\<string, string\> | The field values the person filled in, keyed by `key` (NOTIFY-4). |
| `title` | string | The notification title. |
| `content` | string | The notification body. |
| `severity` | string | `info`, `success`, `warning` or `error`. |

On success a plugin MUST reply with an empty result (`{}`) once the vendor
accepted the message. On failure it MUST reply with an error: `-32003`
(`data.field` naming the field at fault) for anything caused by what the
person entered, `-32000` for anything else (a vendor outage, a network
failure).

## NOTIFY-6

`title` and `content` are plain text the host already translated into the
language the person chose for the channel and rendered with its parameters.
A plugin MUST NOT expect markup in them and SHOULD escape them for the
vendor's own format. A plugin MUST treat an unknown `severity` value as
`info`, so a later spec version can add values.

## NOTIFY-7

A plugin MUST treat `config` values as secret: they MUST NOT appear in a log
line, an error message, or anywhere sent back to the host (SEC-7). This
applies to every value, not only to the fields marked `secret`, since a
webhook URL often embeds a credential.

## `notify.validate`

## NOTIFY-8

Request:

```json
{ "jsonrpc": "2.0", "id": 33, "method": "notify.validate", "params": { "channel": "mychat", "config": { "webhook_url": "not a url" } } }
```

A plugin implementing `notify.validate` MUST check `config` against what the
channel needs, MUST report the first missing or malformed field as `-32003`
with `data.field` set, MUST NOT send anything or otherwise contact the
vendor to do so, and MUST reply `{}` when the configuration is well-formed.

## Host behavior

## NOTIFY-9

A host MUST offer each channel of every enabled `notify` plugin as a
notification channel next to its built-in ones, and MUST stop offering it
once the plugin is disabled or uninstalled. A host MUST keep channel codes
apart from the names of its built-in channels, so a plugin can neither
replace a built-in channel nor be shadowed by one added later; the reference
host stores a plugin channel as `plugin:<code>`. A stored channel whose
plugin is gone stays stored and fails to send until a plugin declaring its
code is enabled again.

## NOTIFY-10

A host MUST render the channel form from `configuration.fields` (NOTIFY-4),
MUST render a `secret` field as a masked input and MUST NOT write a field
value to a log. A host MAY call `notify.validate` before it stores a
configuration and MUST NOT store it when the plugin answers `-32003`; the
reference host does so when a channel is created or changed. The reference
host waits 30 seconds for `notify.send` and 10 seconds for
`notify.validate`; these values are a recommendation.

## NOTIFY-11

A host MUST send only to the plugin that owns the channel `code` at the time
of the call (NAME-10) and starts an `on_demand` plugin to do so. The
reference host does not retry a failed `notify.send`: a notification that
could not be delivered through one channel is still shown in the host and
delivered through its other channels.
