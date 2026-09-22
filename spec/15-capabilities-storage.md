# 15. Capability: `storage`

A plugin declaring `"storage"` in `capabilities` keeps host files in places
the host cannot reach on its own: a WebDAV share, an SFTP server, a cloud
drive. The host offers every backend the plugin declares next to its
built-in storage, and calls the plugin to store, fetch, list and remove
objects there. The reference host uses it as a destination of its automatic
backups. Every method here is a host → plugin request.

| Method | Required | Meaning |
| --- | --- | --- |
| `storage.put` | yes | Store a file under a key. |
| `storage.get` | yes | Fetch the object stored under a key into a file. |
| `storage.list` | yes | List the objects whose key starts with a prefix. |
| `storage.delete` | yes | Remove the object stored under a key. |
| `storage.validate` | no | Check a backend configuration without storing anything. |

A plugin that does not implement `storage.validate` MUST reply with
`-32002` (Unsupported, WIRE-6) or `-32601`, and a host MUST treat both as
"no opinion" rather than as a rejected configuration.

File contents never travel in a message. The host and the plugin exchange
them as files in the exchange directory (STORAGE-5), so a backup of several
gigabytes costs neither side a copy in memory, and the same methods work on
stdio and on gRPC. A `storage` plugin needs no permission to receive these
methods; it SHOULD request `network` (SEC-3), since it talks to the place
where it keeps the objects.

## Manifest block

## STORAGE-1

The manifest MUST include a `storage` block with at least one entry in
`storage.backends` whenever `capabilities` includes `"storage"` (MAN-34):

```json
{
  "storage": {
    "backends": [
      {
        "code": "webdav",
        "name": "WebDAV",
        "configuration": {
          "fields": [
            { "key": "url", "display_name": "Server URL", "help_text": "Folder the objects are kept in", "required": true },
            { "key": "username", "display_name": "Username", "required": true },
            { "key": "password", "display_name": "Password", "required": true, "secret": true }
          ]
        }
      }
    ]
  }
}
```

Each entry of `storage.backends`:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `code` | string | yes | Identifier of the backend on the wire. See NAME-10. |
| `name` | string | yes | Display name of the backend. |
| `configuration.fields` | object[] | no | The fields of the backend form, as in NOTIFY-4. |

## STORAGE-2

`code` MUST match `^[a-z0-9-]{2,32}$` and MUST be unique among the backends
one manifest declares. See `spec/11-naming.md` NAME-10 for cross-plugin
uniqueness.

## STORAGE-3

`name` MUST be a non-empty string. `configuration.fields` follows NOTIFY-4:
the same field shape, the same string encoding of values, the same rules for
`key`, `type`, `display_name`, `required` and `secret`.

## Keys and files

## STORAGE-4

A key names one object. It is a relative path of segments separated by `/`:
it MUST NOT be empty, start or end with `/`, contain an empty segment, a
`.` or `..` segment, a `\` or a control character, and MUST NOT be longer
than 1024 bytes. A host MUST only send keys of this form, and a plugin
SHOULD answer a key of any other form with `-32602`. A plugin maps a key
onto the vendor's own naming as it sees fit, MUST return it unchanged in
`storage.list`, and MUST keep objects of different keys apart; keys are case
sensitive. A key is UTF-8 and may hold any printable character, letters of
any script included, so a plugin whose vendor restricts names maps them
itself, for example by percent-encoding.

## STORAGE-5

Files are exchanged through a directory of the plugin's data directory:
`<NGINX_UI_PLUGIN_DATA_DIR>/exchange/` (LIFE-14). For every call that moves a
file the host creates a fresh subdirectory with a random name that only the
host's user can read, and passes absolute paths inside it:

* For `storage.put` the host places the file at `source_path` before the
  call. The plugin MUST only read it: it MUST NOT modify, move or delete it.
* For `storage.get` the host passes a `target_path` that does not exist yet.
  The plugin MUST create it as a regular file holding the complete object,
  and SHOULD remove a partial file when it fails.

A plugin MUST NOT read or write any other path of the exchange directory,
MUST NOT keep a path after it replied, and MUST NOT rely on the file still
existing after it replied: the host removes the subdirectory once the call
returns, whatever its outcome. A plugin that cannot open `source_path`, or
create `target_path`, replies `-32000`.

## `storage.put`

## STORAGE-6

Request:

```json
{
  "jsonrpc": "2.0", "id": 42, "method": "storage.put",
  "params": {
    "backend": "webdav",
    "config": { "url": "https://dav.example/remote.php/dav/files/alice", "username": "alice", "password": "app-password-xxx" },
    "key": "nginx-ui/daily_1790000000.zip",
    "source_path": "/var/lib/nginx-ui/plugins/.data/io.github.example.webdav/exchange/5f0c2a9d/daily_1790000000.zip"
  }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `backend` | string | The `code` of the backend, without any host prefix. |
| `config` | map\<string, string\> | The field values the person filled in, keyed by `key` (NOTIFY-4). |
| `key` | string | The key to store the file under (STORAGE-4). |
| `source_path` | string | Absolute path of the file to store (STORAGE-5). |

A plugin MUST store the complete file under `key`, replacing an object
already stored under it, and MUST reply only once the object is durable at
the vendor. The reply is `{ "size": <bytes stored> }`. `size` is a double
(WIRE-10), so it stays a JSON number past 4 GiB; it is exact up to 2^53. On
failure a plugin MUST reply with an error: `-32003` (`data.field` naming the
field at fault) for anything caused by what the person entered (a wrong
password, a folder that does not exist), `-32000` for anything else (a
vendor outage, a full disk, a network failure). A failed `storage.put` MUST
NOT leave a partial object under `key` when the vendor makes that possible.

## `storage.get`

## STORAGE-7

Request:

```json
{
  "jsonrpc": "2.0", "id": 43, "method": "storage.get",
  "params": {
    "backend": "webdav",
    "config": { "url": "https://dav.example/remote.php/dav/files/alice", "username": "alice", "password": "app-password-xxx" },
    "key": "nginx-ui/daily_1790000000.zip",
    "target_path": "/var/lib/nginx-ui/plugins/.data/io.github.example.webdav/exchange/9b41e7c0/daily_1790000000.zip"
  }
}
```

A plugin MUST write the complete object stored under `key` to `target_path`
(STORAGE-5) and reply `{ "size": <bytes written> }`. A key that holds no
object is an error: `-32000` with a message saying so. The other errors
follow STORAGE-6.

## `storage.list`

## STORAGE-8

Request:

```json
{
  "jsonrpc": "2.0", "id": 44, "method": "storage.list",
  "params": { "backend": "webdav", "config": { "url": "https://dav.example/remote.php/dav/files/alice", "username": "alice", "password": "app-password-xxx" }, "prefix": "nginx-ui/daily_" }
}
```

Reply:

```json
{ "jsonrpc": "2.0", "id": 44, "result": { "objects": [
  { "key": "nginx-ui/daily_1790000000.zip", "size": 5242880, "modified_at": "2026-09-21T03:00:05Z" }
] } }
```

| Field | Type | Meaning |
| --- | --- | --- |
| `prefix` | string | A plain string prefix of the keys to list, not a directory: `nginx-ui/daily_` matches `nginx-ui/daily_1.zip`. Empty lists every object. |
| `objects[].key` | string | The key, exactly as `storage.put` received it. |
| `objects[].size` | number | Size in bytes, a double as in STORAGE-6. |
| `objects[].modified_at` | string | Last modification as an RFC 3339 timestamp. Empty when the vendor does not tell. |

A plugin MUST list every object whose key starts with `prefix`, following the
vendor's pagination itself, in any order, and MUST reply with an empty list
(or no `objects` member at all, WIRE-10) when nothing matches. A plugin
SHOULD leave out objects it did not store through `storage.put` when it can
tell them apart. The errors follow STORAGE-6.

## `storage.delete`

## STORAGE-9

Request:

```json
{ "jsonrpc": "2.0", "id": 45, "method": "storage.delete", "params": { "backend": "webdav", "config": { "url": "https://dav.example/remote.php/dav/files/alice", "username": "alice", "password": "app-password-xxx" }, "key": "nginx-ui/daily_1780000000.zip" } }
```

A plugin MUST remove the object stored under `key` and reply `{}`. Deleting
a key that holds no object MUST succeed as well, so a host can retry a
cleanup without checking first. The errors follow STORAGE-6.

## `storage.validate`

## STORAGE-10

Request:

```json
{ "jsonrpc": "2.0", "id": 41, "method": "storage.validate", "params": { "backend": "webdav", "config": { "url": "ftp://dav.example", "username": "alice" } } }
```

A plugin implementing `storage.validate` MUST check `config` against what the
backend needs, MUST report the first missing or malformed field as `-32003`
with `data.field` set, MUST NOT store, change or remove anything, and MUST
reply `{}` when the configuration is well-formed. It SHOULD NOT contact the
vendor; a host that wants to know whether the vendor accepts the
configuration calls `storage.list` instead (STORAGE-13).

## STORAGE-11

A plugin MUST treat `config` values as secret, as NOTIFY-7 describes, and
MUST NOT write the contents of a file it stores or fetches to a log.

## Host behavior

## STORAGE-12

A host MUST offer each backend of every enabled `storage` plugin next to its
built-in storage, MUST keep its built-in storage the default, and MUST keep
backend codes apart from the names of its built-in storage; the reference
host stores a plugin backend as `plugin:<code>` in the `storage_type` of an
automatic backup, next to the built-in `local` and `s3`. A host MUST render
the backend form from `configuration.fields` as NOTIFY-10 describes, MUST
NOT store a configuration that leaves a `required` field empty, MUST store
the values encrypted at rest, and MUST call only the plugin that owns the
backend `code` at the time of the call (NAME-10), starting an `on_demand`
plugin to do so. A host MAY call `storage.validate` before it stores a
configuration and MUST NOT store it when the plugin answers `-32003`.

## STORAGE-13

A host MUST create the exchange subdirectory of STORAGE-5 inside the data
directory of the plugin it calls, never elsewhere, MUST give it a name that
cannot be guessed, MUST remove it once the call returned or failed, and
SHOULD remove leftovers of a previous run it finds there. The reference host
waits 10 minutes for `storage.put` and `storage.get` and 30 seconds for the
other methods; these values are a recommendation. A host MUST report a
failed call as a failure of the operation that needed it and MUST NOT treat
an object as stored unless `storage.put` succeeded.

## STORAGE-14

The reference host uses a storage backend for automatic backups as follows.
The `storage_path` of the backup task is the key prefix, and each run stores
the archive as `<prefix>/<name>_<unix time>.zip`, plus
`<prefix>/<name>_<unix time>.zip.key` for an encrypted backup. A task may
keep only its latest backups: after a successful run the host calls
`storage.list` with the task's prefix and `storage.delete` for the runs past
the limit. Restoring a stored backup calls `storage.get` for the archive and
its key file. The "test" action of the backup form calls `storage.validate`
and then `storage.list` with the task's prefix. A failure of any of these is
reported on the backup task; nothing is retried automatically.
