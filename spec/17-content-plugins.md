# 17. Content plugins

A plugin can contribute content instead of, or next to, code: nginx
configuration templates and translation files. It declares them in the
`content` block of its manifest (MAN-18), as directories relative to the
package root:

```json
{
  "id": "io.github.example.snippets",
  "name": "Extra snippets",
  "version": "1.0.0",
  "api_version": 1,
  "content": {
    "templates": "templates",
    "locales": "locales"
  }
}
```

```
io.github.example.snippets/
├── plugin.json
├── README.md
├── templates/
│   ├── block/
│   │   └── cache-static.conf
│   └── conf/
│       └── ghost.conf
└── locales/
    ├── de_DE.po
    └── zh_CN.po
```

The host reads these files while the plugin is enabled and stops using them
once it is disabled, uninstalled or upgraded to a version whose permissions
still need approval. Content is data: nothing in it is executed as a program,
and a plugin that declares only `content` has no process at all.

## Process-less plugins

## CONTENT-1

A plugin whose manifest has no `server` block has no process. A host MUST NOT
start a process for it, send it any JSON-RPC message or wait for a handshake,
and treats it as running for as long as it is enabled; enabling and disabling
it only adds and removes its contributions. Because every capability,
manifest cron entry and event subscription is served by the process, a
manifest without `server` MUST NOT declare `capabilities`, `cron` or
`events`. A plugin MAY declare `content` next to `server` and `webapp`; its
content then follows the same rules.

## Templates

## CONTENT-2

`content.templates` names a directory with up to two subdirectories:
`conf/` for server level templates and `block/` for snippets a person adds
to a server block, the same two lists the host offers for its built-in
templates. The directory MUST exist, and at least one of the two
subdirectories MUST contain a template. A template is a regular file whose
name matches `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}\.conf$`; a host ignores every
other entry, and a linter SHOULD warn about one.

## CONTENT-3

A template file MUST use the format of the host's built-in templates:

```
# Nginx UI Template Start
name = "Cache static files"
author = "@example"
description = { en = "Cache images, scripts and styles", de_DE = "Bilder, Skripte und Stylesheets zwischenspeichern" }

[variables.expires]
type = "string"
name = { en = "Expires", de_DE = "Ablauf" }
value = "30d"
# Nginx UI Template End
location ~* \.(?:css|js|png|jpe?g|gif|svg|webp)$ {
    expires {{.expires}};
    add_header Cache-Control "public";
}
```

* The first line is `# Nginx UI Template Start`, and a later line is
  `# Nginx UI Template End`. The lines between them are a TOML document with
  `name` (string), `author` (string), `description` (a table of texts keyed
  by language code, CONTENT-6) and `variables`.
* Each entry of `variables` has a `type` (`string`, `boolean` or `select`),
  a `name` table keyed by language code, a default `value` of that type and,
  for `select`, a `mask` table mapping every choice to its labels.
* Everything after the end line is the body: a Go `text/template` whose
  output is nginx directives valid inside a `server` block. A section between
  `# Nginx UI Custom Start` and `# Nginx UI Custom End` is rendered the same
  way and added to the custom directives of the server verbatim. The data of
  both is the value of every variable under its key, plus `HTTPPORT` and
  `HTTP01PORT`, the ports the host listens on.

The body and the custom section MAY use the actions `{{.name}}`, `if`,
`else` and `with`, and the functions `and`, `or`, `not`, `eq`, `ne`, `lt`,
`le`, `gt`, `ge`, `len`, `index`, `print`, `println`, `html`, `js` and
`urlquery`, which is everything the built-in templates need. A host MUST
reject a plugin template that uses anything else (`range`, `define`,
`template`, `block`, `printf`, `call`): a loop or a format width can make
rendering unbounded. A host MAY bound the size of a template and of what it
renders to; the reference host accepts 256 KiB per file and 1 MiB per
rendered section.

`name` SHOULD be present; a template without it is listed under its file
name.

## CONTENT-4

A host MUST validate every template of a package before it installs or
upgrades the plugin, and MUST refuse the package when one does not parse:
the start or end line is missing, the header is not valid TOML of the shape
CONTENT-3 describes, the body or the custom section is not a valid template
or fails to render with the default values, or the rendered body is not valid
nginx syntax. The reference host applies the parser it uses for its built-in
templates.

## CONTENT-5

A host MUST offer the templates of every enabled plugin in the same lists as
its built-in templates and MUST tag each with the plugin it comes from; the
reference host adds `origin` (`"builtin"` or `"plugin"`) and `plugin_id` to
every entry of `GET /api/templates/configs` and `GET /api/templates/blocks`,
and addresses a plugin template as `GET` or `POST
/api/templates/block/<file>?plugin_id=<id>`. A plugin template MUST NOT
replace a built-in one or the template of another plugin, whatever its file
name, and MUST disappear from the lists once the plugin is disabled.

## Locales

## CONTENT-6

`content.locales` names a directory of translation files, one per language,
named `<lang>.po`. `<lang>` MUST be a language the host translates its own
interface into; for the reference host these are `en`, `zh_CN`, `zh_TW`,
`fr_FR`, `es`, `de_DE`, `ru_RU`, `vi_VN`, `ko_KR`, `tr_TR`, `ar`, `uk_UA`,
`ja_JP` and `pt_PT`. The directory MUST exist and contain at least one such
file. A host ignores every other entry, and a linter SHOULD warn about one.

## CONTENT-7

A translation file MUST be a GNU gettext PO file in UTF-8 whose first entry
is the header (`msgid ""`), with a `msgstr` for every `msgid`. A host MUST
validate every translation file of a package before it installs or upgrades
the plugin and MUST refuse the package when one does not parse. Entries
flagged `fuzzy` are ignored, as for the host's own catalogs.

## CONTENT-8

While a plugin is enabled, a host MUST merge its entries into the catalog it
serves for the language of the file, so text a plugin shows through the
host's translation function (its webapp, the names in its templates) is
translated without further work. A host MUST NOT let a plugin change a
translation of its own: when the host catalog already translates a `msgid`
(with a non-empty `msgstr`), the host entry wins. When several plugins translate the same `msgid`, the
plugin with the lowest id wins. A host MUST remove the entries of a plugin
once it is disabled, uninstalled or upgraded to a version whose permissions
still need approval. The reference host serves the merged catalog at
`GET /api/translation/<lang>`.

## Security

## CONTENT-9

Content is untrusted input. A host MUST NOT execute anything a content file
contains beyond rendering a template with the data CONTENT-3 lists, MUST
resolve every content path inside the plugin directory (PKG-3), and MUST add
the directives of a plugin template to a configuration only when a person
picks the template and after showing them the directives it renders to. A
translation replaces display text only; a host MUST keep treating translated
text as text, never as markup or code.
