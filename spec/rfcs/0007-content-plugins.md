# RFC 0007: Content plugins

| | |
| --- | --- |
| Status | Accepted, implemented |
| Date | 2026-09-23 |
| Spec changes | `spec/17-content-plugins.md` (CONTENT-1 through CONTENT-9), MAN-7, MAN-18, LIFE (process-less note), CONF-1, CONF-4, CONF-13, the lint table of `spec/09-conformance.md`, `ManifestContent` comments in `manifest.proto`, `schema/plugin.schema.json` |
| Reference host | nginx-ui `internal/template` (plugin template source, `ValidateFile`), `internal/translation` (plugin catalogs, `ParsePO`, `Languages`), `internal/plugin/content.go` (`ContentEntries`, install validation), `internal/plugin/capability/content.go`, `internal/plugin/lint.go`, `internal/plugin/conformance.go`, `api/template`, `app/src/views/site/site_edit/components/ConfigTemplate` |
| Reference SDK | none: a content plugin has no process |

## Summary

The `content` block of the manifest has existed since the first version of
the spec, with two directories, `templates` and `locales`, and no rule for
what they hold or what a host does with them. This RFC specifies both. A
templates directory holds `conf/` and `block/` templates in the format of the
host's built-in templates, which the host lists next to its own while the
plugin is enabled, tagged with the plugin. A locales directory holds one
gettext `.po` file per host language, which the host merges into the catalog
it serves, without ever overriding a translation of its own. A plugin that
declares only `content` has no process.

## Motivation

Config templates are the most requested kind of contribution: snippets for
an application, a framework or a hardening recipe. Today each one has to be
added to the host. Translations of plugin interfaces had to be shipped in a
webapp bundle through `registerTranslations`, which a plugin without a
bundle cannot do. Both are plain files, so a plugin that only ships them
should not need a process, a build or an SDK.

## Design

### Process-less plugins

A manifest without `server` has no process (CONTENT-1): the host starts
nothing, sends nothing and reports the plugin running while it is enabled.
Every capability, manifest cron entry and event subscription is served by a
process, so such a manifest declares none of them; the reference host
rejects the manifest otherwise.

### Templates

The format is the one of the host's built-in templates (CONTENT-3): a TOML
header between two marker lines with a name, an author, localized
descriptions and typed variables, followed by a Go `text/template` body that
renders to nginx directives, with an optional custom section. Reusing it
keeps one parser, one editor dialog and one mental model for authors. The
host validates every template before installing a package, with the parser
it uses for its own (CONTENT-4), and lists plugin templates next to its own
with an `origin` and a `plugin_id`; a plugin template is addressed by plugin
id and file name, so it can neither replace a built-in template nor another
plugin's.

### Locales

One `<lang>.po` per language the host has (CONTENT-6), in the gettext format
the host already uses for its own catalogs. The host merges the entries into
the catalog it serves while the plugin is enabled (CONTENT-8). A built-in
translation always wins, so a plugin cannot change what the host's own
interface says; among plugins the lowest id wins, as for capability codes.
The reference host serves the merged catalog at `/api/translation/<lang>`,
which the web interface loads at start, so a plugin webapp or a plugin
template name is translated without a bundle of its own.

### Reference host

* `internal/template` keeps a registry of template sources next to the
  embedded templates. `internal/plugin/capability.RegisterContent` registers
  the plugin manager as one: every enabled, approved plugin with
  `content.templates` contributes its `conf/*.conf` and `block/*.conf`.
  `ConfigInfoItem` gains `origin` and `plugin_id`, and the block endpoints
  take `?plugin_id=`; a plugin or file that is not there is error 56001
  ("template not found"). The site editor shows a "from plugin" tag and
  reminds the person to review the directives of a plugin template.
* A plugin template renders with the same code as a built-in one, restricted
  to the actions and functions CONTENT-3 lists, with 256 KiB per file and
  1 MiB per rendered section. A template is read only as a regular file in a
  real `conf/` or `block/` directory; symlinks are ignored.
* `internal/template.ValidateFile` applies the header, template and nginx
  parsers to one file; `internal/plugin` runs it on every template of a
  package before installing it and in `nginx-ui plugin lint`.
* `internal/translation` keeps the embedded catalogs apart from a plugin
  layer. A built-in entry without a translation (an empty `msgstr`) counts
  as untranslated, so a plugin may fill it. `GetTranslation` merges them per language (built-in first, plugins
  in id order, first one wins) and caches the result until the plugin layer
  changes. `RegisterContent` reloads the plugin layer whenever the plugin
  inventory changes (`plugin.changed`). `ParsePO` parses a catalog safely;
  `Languages` is the allowlist of CONTENT-6, read from the host's own
  language list.
* Installing a package whose templates or catalogs do not parse fails with
  error 55021 ("plugin content {0} is invalid: {1}").
* `nginx-ui plugin conformance` runs the static checks of CONF-13, and only
  those, for a plugin without `server`.

## Compatibility

`content` keeps its shape; only rules for its files are added (VER-1). A
package that declared `content` with files that do not follow them was never
used by any host, since none read them.

## Alternatives considered

* **A new template format with a JSON schema.** Two formats for the same
  dialog, and authors of built-in templates could not reuse their work.
* **Letting plugin translations override the host.** A plugin could then
  change what the host's own buttons and warnings say.
* **Loading translations only through the webapp registry.** Needs a bundle
  and a build step for what is a text file.

## Security considerations

Templates and catalogs are data. A template is rendered with Go
`text/template`, which has no function that reads files or runs commands,
with the variables of the template and two port numbers as its only data,
and its directives reach a configuration only when a person picks the
template in the editor, after seeing them. Paths are resolved inside the
plugin directory (PKG-3). A translation can change display text of plugin
strings only, never of the host's own, and is rendered as text.

## Future work

* Stream templates, and templates for the `http` block.
* Translations of the manifest's own display names (channel and kind names).
