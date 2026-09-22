# schema

`plugin.schema.json` is the JSON Schema (draft 2020-12) of `plugin.json`,
for plugin authors, editors and `nginx-ui plugin lint`.

## Source

The structure of the manifest is defined by the `Manifest` message in
`proto/nginxui/plugin/v1/manifest.proto`; `plugin.json` is its protobuf JSON
mapping with proto field names. The schema is written by hand because it
carries validation rules the proto cannot express: required members, id and
version patterns, enums, path restrictions and the rules that tie a
capability to its metadata block.

The schema is verified against the proto, so the two cannot drift silently.
`tools/schema` (run by `make check`) walks the schema from its root, following
`$ref`, and asserts for the root object and every nested object that:

* its `properties` are exactly the field names of the proto message at the
  same position,
* each property's JSON `type` matches the proto field (string, integer,
  boolean, array for repeated fields, object for maps and messages),
* every object definition under `$defs` is reachable from the root.

The same package also decodes every `examples/*/plugin.json` into `Manifest`.

## Changing the manifest

1. Add the field to `manifest.proto` and run `make generate`.
2. Add the property to `plugin.schema.json` with its validation rules.
3. Describe it in `spec/01-manifest.md` and run `make check`.
