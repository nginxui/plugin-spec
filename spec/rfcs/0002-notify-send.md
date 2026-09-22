# RFC 0002: The `notify` capability

| | |
| --- | --- |
| Status | Accepted, implemented |
| Date | 2026-09-23 |
| Spec changes | `spec/12-capabilities-notify.md` (NOTIFY-1 through NOTIFY-11), MAN-19, MAN-31, NAME-10, CONF-8, WIRE-6 (`-32002` row), WIRE-9 table, `proto/nginxui/plugin/v1/notify.proto`, `ManifestNotify` and `ConfigurationSchema` in `manifest.proto`, `schema/plugin.schema.json`, vectors 23 through 26 |
| Reference host | nginx-ui `internal/plugin/capability/notify.go`, `internal/notification/source.go`, `api/external_notify` (`GET /api/external_notifies/channels`, validation on create and modify), `app/src/components/PluginConfigForm`, `app/src/views/preference/components/ExternalNotify` |
| Reference SDK | nginx-ui-plugin-sdk-go `notify.go` (`NotifyHandler`, `NotifyValidator`) |

## Summary

A plugin may deliver host notifications through vendor channels. It declares
the channels in a `notify` block of its manifest, each with a code, a name and
the fields of its configuration form. The host offers the channels next to
its built-in ones, renders their forms from the schema, stores the values
like any other channel configuration, and calls `notify.send` with the
already translated title and body whenever a notification is routed to one.
`notify.validate` optionally checks a configuration before it is stored.

## Motivation

The host ships eleven built-in notification channels, each one a Go type and
a generated form. Every new vendor means a host release, and vendors that
only a few people use (an internal paging gateway, a regional chat service)
never make it in. The notification registry itself is already open: channels
register a handler by name and the settings page lists them. A plugin only
needs a way to become another source of channels, which is what this
capability adds, without moving any built-in channel out of the core.

## Design

### Manifest

```json
{
  "capabilities": ["notify"],
  "permissions": ["network"],
  "notify": {
    "channels": [
      {
        "code": "mychat",
        "name": "MyChat",
        "configuration": {
          "fields": [
            { "key": "webhook_url", "display_name": "Webhook URL", "required": true },
            { "key": "token", "display_name": "Token", "required": true, "secret": true }
          ]
        }
      }
    ]
  }
}
```

The form schema, `ConfigurationSchema`, is shared with the `probe` capability
(RFC 0003). It is deliberately smaller than `settings_schema`: values travel
as `map<string, string>`, the map the host already stores for every channel,
so the field types are limited to `text`, `textarea`, `number` and `bool`,
and `secret` is a flag on a text field instead of a type of its own. A
channel code follows the provider code rules: `^[a-z0-9-]{2,32}$`, one
namespace across every installed `notify` plugin (NAME-10).

### Methods

| Method | Params | Result |
| --- | --- | --- |
| `notify.send` | `{ channel, config, title, content, severity }` | `{}` |
| `notify.validate` | `{ channel, config }` | `{}`, or `-32003` with `data.field` |

`severity` is the host notification type: `info`, `success`, `warning`,
`error`. The host translates `title` and `content` into the language the
person chose for the channel before the call, so a plugin never needs the
host's translation catalog.

### Reference host

* `internal/notification` gains `ExternalNotifierSource`, a source of
  notifier types consulted after the built-in registry. Built-in types always
  win, so a plugin cannot take over a built-in channel.
* `internal/plugin/capability.RegisterNotify` registers the plugin manager as
  such a source. A channel is exposed as the notifier type `plugin:<code>`:
  the prefix keeps plugin codes apart from the built-in names (NOTIFY-9), so a
  later built-in channel can never shadow a configured plugin channel. The
  owning plugin is resolved at every call (lowest plugin id when several
  declare one code), and an `on_demand` plugin is started for it.
* `GET /api/external_notifies/channels` lists the plugin channels with their
  field schema. The settings page adds them to the channel type selector and
  renders their form with the generic `PluginConfigForm` component; the
  eleven built-in forms stay generated and unchanged.
* Creating a channel configuration, or changing its type or values, calls
  `notify.validate`. Only `-32003` blocks the save, reported as error 400004
  ("notifier config field {0} is invalid: {1}"); a plugin that does not
  implement the method, fails or cannot be started has no opinion.
* `notify.send` has 30 seconds, `notify.validate` 10. Delivery failures are
  not retried, as for the built-in channels.

## Compatibility

A new optional capability, block and pair of rpcs; nothing existing changes
meaning (VER-1). The `notify` permission, which gates `host.notify`, is
unrelated and unchanged. Stored channel configurations of a plugin that is
later disabled stay stored and fail to send until the plugin is back.

## Alternatives considered

* **Storing the bare code as the notifier type.** Simpler, but a built-in
  channel added later under the same name would silently take over every
  configured plugin channel.
* **Reusing `settings_schema` for the form.** Its `select` and `default`
  need typed values the channel map cannot carry, and its `secret` type does
  not combine with `textarea`. A dedicated schema keeps the wire honest.
* **Letting the plugin render its own form through a webapp slot.** Possible
  later, but it forces every notification plugin to ship a browser bundle for
  what is almost always a handful of text fields.

## Security considerations

Channel values often embed credentials (tokens, webhook URLs with a secret
path). The reference host stores them encrypted at rest like every other
channel configuration, masks `secret` fields in the form and never logs the
values; a plugin must not log them either (NOTIFY-7). Like the built-in
channels, the reference host returns stored values to an authenticated
administrator in the channel list; SEC-8 style redaction of channel values is
left to a later revision because it would also change the built-in channels.
Creating or changing a channel keeps requiring an interactive administrator
with a secure session.

## Future work

* The go-to link the built-in channels render (`url`) as an optional field of
  `notify.send`.
* Server-side redaction of `secret` field values, for built-in and plugin
  channels alike.
* `select` fields with options, once a typed value map exists.
