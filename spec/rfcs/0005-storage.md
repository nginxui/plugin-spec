# RFC 0005: The `storage` capability

| | |
| --- | --- |
| Status | Accepted, implemented |
| Date | 2026-09-23 |
| Spec changes | `spec/15-capabilities-storage.md` (STORAGE-1 through STORAGE-14), MAN-19, MAN-34, NAME-10, CONF-4, CONF-7, CONF-11, WIRE-6 (`-32002` and `-32602` rows), WIRE-9 table, WIRE-10 (`double` byte sizes), WIRE-11 routing table, LIFE-10 (shutdown list), `proto/nginxui/plugin/v1/storage.proto`, `ManifestStorage` in `manifest.proto`, `schema/plugin.schema.json`, vectors 33 through 38 |
| Reference host | nginx-ui `internal/plugin/capability/storage.go`, `internal/backup/storage.go`, `internal/backup/storage_source.go`, `model.AutoBackup` (`storage_config`, `retention_count`), migration `20260923000002`, `api/backup` (`GET /api/auto_backup/storage_backends`, `POST /api/auto_backup/test_storage`, `/api/auto_backup/:id/stored`), `app/src/views/backup/AutoBackup` |
| Reference SDK | plugin-sdk-go `storage.go` (`StorageHandler`, `StorageValidator`, `StoredObject`) |

## Summary

A plugin may keep host files in storage the host cannot reach on its own. It
declares storage backends in a `storage` block, each with a code, a name and
a configuration form, and implements `storage.put`, `storage.get`,
`storage.list` and `storage.delete` over plain keys, plus an optional
`storage.validate`. File contents never travel in a message: the host places
and picks up files in an exchange directory inside the plugin's own data
directory and passes their paths. The reference host offers the backends as
destinations of its automatic backups next to its built-in local and S3
storage.

## Motivation

Automatic backups can go to a local directory or to S3, both hard-coded in
the backup package. People ask for WebDAV, SFTP, cloud drives and rclone
remotes; each is a sizable dependency most installations never use, and some
of them change their APIs often. The backup code already separates "make an
archive" from "put it somewhere", so a plugin only needs to supply the
second half. The same four verbs cover later uses such as archiving logs.
S3 stays in the core: it is what most people use and restoring from it must
not depend on a plugin being installed.

## Design

### Manifest

```json
{
  "capabilities": ["storage"],
  "permissions": ["network"],
  "storage": {
    "backends": [
      {
        "code": "webdav",
        "name": "WebDAV",
        "configuration": {
          "fields": [
            { "key": "url", "display_name": "Server URL", "required": true },
            { "key": "username", "display_name": "Username", "required": true },
            { "key": "password", "display_name": "Password", "required": true, "secret": true }
          ]
        }
      }
    ]
  }
}
```

The form reuses `ConfigurationSchema` from RFC 0002, and backend codes follow
the same rules as channel codes, in a namespace of their own (NAME-10).

### Methods

| Method | Params | Result |
| --- | --- | --- |
| `storage.put` | `{ backend, config, key, source_path }` | `{ size }` |
| `storage.get` | `{ backend, config, key, target_path }` | `{ size }` |
| `storage.list` | `{ backend, config, prefix }` | `{ objects: [{ key, size, modified_at }] }` |
| `storage.delete` | `{ backend, config, key }` | `{}`, also for a missing key |
| `storage.validate` | `{ backend, config }` | `{}`, or `-32003` with `data.field` |

Keys are relative `/` separated paths without empty, `.` or `..` segments
(STORAGE-4); `prefix` is a plain string prefix, as in S3. `modified_at` is an
RFC 3339 string. `size` is a `double`: an `int64` would be a JSON string under
the protobuf mapping (WIRE-10) and `uint32` stops at 4 GiB, which a
directory backup can pass. A double is a plain JSON number and exact up to
2^53 bytes.

### Exchange directory

For each `storage.put` and `storage.get` the host creates
`<NGINX_UI_PLUGIN_DATA_DIR>/exchange/<random>/` with mode 0700, places the
source file there (a hard link when the file system allows it, a copy
otherwise) or names the target file there, makes the call, moves the fetched
file out and removes the directory whatever the outcome. The plugin never
needs access outside its own data directory, the host never trusts a path
the plugin names, and neither side holds a whole archive in memory, on
stdio as on gRPC.

### Reference host

* `internal/backup` gains a storage backend registry (`StorageSource`,
  `RegisterStorageSource`, `StorageBackends`). The built-in `local` and `s3`
  are listed first and stay the default; their code paths are unchanged.
* `internal/plugin/capability.RegisterStorage` registers the plugin manager as
  a source. A backend is stored as `plugin:<code>` in the `storage_type` of
  an automatic backup, for the reason RFC 0002 gives, and the owning plugin is
  resolved at every call, starting an `on_demand` plugin.
* `model.AutoBackup` gains `storage_config`, the form values of a plugin
  backend, encrypted at rest like the S3 credentials, and `retention_count`,
  how many runs a plugin backend keeps (0 keeps all). Migration
  `20260923000002` adds both columns; existing rows keep NULL, which the code
  reads as no values and no limit.
* A run builds the archive in a temporary directory, stores it and its key
  file under `<storage_path>/<file name>` with `storage.put`, removes the
  local files, and then applies `retention_count`: it lists the task's
  prefix, groups the archive and key file of each run by their timestamp and
  deletes the oldest runs past the limit. A failed cleanup is logged and does
  not fail the run.
* Creating or changing a task checks the required fields and calls
  `storage.validate`; only `-32003` blocks the save, reported as error 55202
  ("storage config field {0} is invalid: {1}"). A task whose plugin is gone
  fails its runs with error 55201 ("storage backend {0} is not available")
  until the plugin is back.
* `GET /api/auto_backup/storage_backends` lists the built-in and plugin
  backends with their forms, `POST /api/auto_backup/test_storage` runs
  `storage.validate` and `storage.list`, and `/api/auto_backup/:id/stored`
  lists the runs a plugin backend keeps (`storage.list`), deletes one
  (`storage.delete`) and restores one (`storage.get` of the archive and its
  key file, then the same restore as an uploaded backup).
* The backup form offers the plugin backends in the storage type selector
  when any exists, renders their form with `PluginConfigForm`, and asks for
  the key prefix and the number of runs to keep. The task list has a "Stored
  backups" dialog for tasks on a plugin backend.
* `storage.put` and `storage.get` have 10 minutes, the other methods 30
  seconds.

## Compatibility

A new optional capability, block and service; nothing existing changes
meaning (VER-1). The WIRE-10 note on `double` sizes only adds a type. The
new database columns are nullable and omitted from the MCP view of a backup
task.

## Alternatives considered

* **Streaming the file in chunks over JSON-RPC.** Works on stdio, but costs
  base64 overhead, needs a chunk protocol with its own flow control and a
  second shape on gRPC. Files on a shared disk are simpler and faster.
* **Letting the plugin read the host's temporary directory.** It would widen
  what a plugin may touch to the whole host temporary directory and makes
  cleanup the plugin's job. The exchange directory keeps both sides inside
  the plugin's data directory, which the host already scopes to it (SEC-2).
* **Moving S3 into a plugin.** Rejected, see Motivation.
* **A retention setting for every storage type.** The built-in backends keep
  their behavior in this change; a shared retention setting can follow once
  it covers local and S3 as well.

## Security considerations

A storage plugin sees the backups it stores, which contain the host's
configuration and, for an encrypted backup, the key file next to it, the
same exposure as the S3 backend. Backend values are credentials; the
reference host encrypts them at rest, masks `secret` fields, leaves them out
of the MCP view and never logs them, and a plugin must not log them either
(STORAGE-11). Changing a task keeps requiring a secure session; running,
testing and restoring stay unavailable in demo mode.

## Future work

* Retention for the built-in local and S3 storage.
* Log archiving to a storage backend.
* A resumable upload for very large archives, if a vendor needs one.
