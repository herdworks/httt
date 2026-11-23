# httt

The `github.com/herdworks/httt` package is the HyperTemplates Core templating engine.

## Iteration

Use `ht-each` (or `data-ht-each` in strict mode) to repeat an element from one
explicit typed source:

```html
<article ht-each="post in ${ posts, archive.posts }">
  <h2 ht-apply>${ post.title }</h2>
</article>
```

Sources must be a complete `${ ... }` lookup expression. Arrays and slices
produce one clone per item; every other non-nil value produces one clone.
`ht-offset` and `ht-limit` accept literal non-negative integers. Each clone has
an isolated top-level data context with the loop target plus `each.index` (one
based source index), `each.count` (the emitted window count), and `each.total`
(the unwindowed source count). Unchanged nested values are read-only shared
render input. `each` is reserved as loop-local iteration state and cannot be
used as a loop binding.

`ht-template` and `data-ht-template` are no longer runtime directives. Run
`hyperctl theme migrate` before rendering themes that still use them.

## Local bindings

Use one or more `ht-with` declarations to give complete `${ ... }` lookups
short, local names. `ht-with` processes its target once as a nested template,
so its bindings are available throughout that target and its descendants. The
exact syntax is `${ variable } as binding`; bindings run in source order, so a
later one can reference an earlier name:

```html
<example-fragment
  ht-include="fragments/example"
  ht-with="${ page.foo } as foo"
></example-fragment>
```

`ht-with` is the single-pass counterpart to `ht-each`: it has no `each.*`
metadata and binds its values exactly once, including arrays and maps. Both
directives create a local data scope that never mutates outer render data.
`ht-with` and `ht-each` cannot be combined on one element. `each` remains
reserved and cannot be used as a binding.

## Blocks

`ht-block` is the directive intended for HTML supplied by content/template-data
files. Blocks run once after the containing document has rendered its ordinary
directives. A block template receives the invoking element's attributes and
child content under `block.*`, plus only the caller-provided safe data context.
For example, Hypertexting supplies its page data as `page.*`; HyperTemplates
itself does not reserve or interpret that namespace.

## Legacy content and parameter migration

`ht-content`, `data-ht-content`, `ht-param`, and `data-ht-param` are no longer
runtime directives. `hyperctl theme migrate` converts supported content and
parameter usages to `ht-apply` template output. For plain or `html:` content,
the source and text fallback become direct output; `markdown:` becomes
`markdown(...)`:

```html
<title ht-content="page.title,site.title">Placeholder</title>
<!-- becomes -->
<title ht-apply>${ page.title, site.title, "Placeholder" }</title>
```

Elements with multiple content directives, multiple legacy declarations, or
non-text fallback content are reported for manual migration.

## Migrating themes with legacy HyperTemplates directives

Use `hyperctl theme migrate --interactive --theme <theme-dir>` to review and apply safe,
idempotent conversions for legacy `ht-not`, `ht-template`, `ht-attr`,
`ht-param`, `ht-content`, and `ht-query` declarations. Legacy push-based
`ht-pipe` declarations are reported for manual migration because a source-side
directive cannot safely determine its destination or source selector. The command
can write a complete migrated copy with `--output <directory>` and uses
interactive review with `--interactive`; unsupported or ambiguous declarations
are left unchanged and reported with their locations.

The removed runtime directives above fail with a migration hint. `ht-attrs`
remains active: each repeated value must be one complete `${ ... }` expression
that resolves to a string-keyed attribute map, and later maps win conflicts.
Use `ht-apply` for ordinary templated attributes and text. `ht-base` requires
repeatable `target on ${ value }` declarations, while `ht-pipe` annotates a
destination with a quoted literal source selector such as
`from "style.component" as css`.

## Packaging and releasing themes

Use the selected theme's `theme.json` as the source of its name and version:

```sh
httt theme package -c site.yaml -t themes/example/theme.json -f example.zip
httt theme release -c site.yaml -t themes/example/theme.json
```

`httt package` remains supported with its existing options and output behavior.
`httt release` is the same release operation as `httt theme release`. Consumers
that mount the HTTT command tree at `hyperctl theme` retain
`hyperctl theme package` and gain `hyperctl theme release` when they choose to
update their HTTT dependency. No consumer dependency is updated automatically.

Release prepares local files only. It does not upload, deploy, commit, tag, bump
versions, or install updates. Set the desired version in `theme.json` first;
missing name/version metadata is rejected rather than filled with display
fallbacks. The release index describes one theme per website; an existing index
for a different theme is rejected.

With the default site static directory, a theme named `Example Theme` at version
`1.2.3` produces:

```text
static/.hypertexting/themes/example-theme-v1.2.3.zip
static/.hypertexting/themes.json
```

The slug is the lowercased theme name with runs of non-ASCII-alphanumeric
characters replaced by `-`, trimmed at both ends. Versions must start with a
letter or digit and contain only letters, digits, `.`, `+`, `_`, or `-`.
The site's `config.static_dir` controls the output root (default `static`),
relative to `site.yaml`. The selected theme's own `config.static_dir` continues
to identify its packaged static resources. The `-t` path is relative to the site
configuration directory, as with `package`.

For a different local archive location, use
`--file downloads/example-v1.2.3.zip` on `release`. Unlike `package --file`, this
path is **relative to the site's static directory**. The index always stays at
`.hypertexting/themes.json`. The emitted archive URL is relative to the website
root and is escaped for use as a URL.

The canonical single-theme index has this shape:

```json
{
  "name": "Example Theme",
  "latest": "1.2.3",
  "versions": {
    "1.2.3": {
      "url": "/.hypertexting/themes/example-theme-v1.2.3.zip",
      "sha256": "<64 hexadecimal characters: SHA-256 of the exact ZIP bytes>"
    }
  }
}
```

Archives use the native `Theme.Archive` packager, including configured resource
directories and root `theme.json`. Release stages the archive, verifies its ZIP
entries and metadata, hashes it, installs it without replacing an existing
file, and verifies the installed digest before atomically replacing the index.
Existing version records and extra index metadata are preserved; `latest` is
set to the version selected by this invocation. An identical-byte retry is
allowed only at the same archive path. A different path for an indexed version
is rejected even when its bytes match; choose custom archive locations on the
initial release. Equivalent root-relative and index-relative URL spellings are
compared against the canonical index path, and an existing URL spelling is
preserved on retry. Host, query, and encoded-path differences are not treated as
equivalent local output locations. Different bytes for an existing version or
output file are rejected without changing the index. ZIP metadata,
including source modification times, contributes to those bytes.

If the theme's packaged static resources overlap the site's static directory,
release excludes only the exact manifest and lock files, this invocation's
staging directories, the selected archive file, and earlier local archive files
recorded in the index. Other legitimate assets under `.hypertexting`, including
unrelated ZIP files, remain packaged. This prevents releases from packaging
themselves on retries or later versions without discarding theme assets. Keep
the index so earlier archive paths remain known. External archive URLs do not
imply local paths.

Release rejects a symlink used as the static root, and symlink destinations or
ancestors within the static tree for the manifest, lock, current archive, and
prior local archives. Ordinary nested directories remain supported. Output
reads, writes, archive installation, and index replacement are confined to the
static directory through `os.Root`. Native packaging runs in private scratch
space before its finished bytes are copied into a confined output stage.

An interrupted index write may leave a valid unindexed archive; retry with the
same source files and bytes. Concurrent releases are rejected by a local
`.hypertexting/.theme-release.lock`. If a process is forcibly stopped, remove
its leftover lock only after confirming no release is running.

### Resolving releases from an installed theme

An installed theme's `config.url` identifies its release index:

```json
{
  "name": "Example Theme",
  "version": "1.2.3",
  "config": { "url": "https://themes.example" }
}
```

Both `https://themes.example` and `https://themes.example/` resolve to
`https://themes.example/.hypertexting/themes.json`. An explicit non-root path,
such as `https://themes.example/catalog/releases?channel=stable`, is fetched as
supplied. HTTT provides `theme.ReleaseIndexURL`, `theme.FetchReleaseIndex`, and
`(*theme.Theme).ResolveRelease(ctx, client, version)` for this shared behavior.
Pass an empty version to select `latest`, and a nil HTTP client to use the
default client. Loading `theme.json` stays offline; resolving releases is an
explicit operation and does not install or change anything.

Clients must use the selected version's `url` and `sha256` from the index.
Archive paths are not a client convention: URLs may point to another directory
or an external HTTPS host. Relative URLs resolve against the fetched index URL,
including its final location after redirects. The resolver validates the index,
checks its theme name against the installed theme, and returns the resolved
archive URL and digest. Callers downloading the archive must verify its bytes
against that digest before installation. No existing consumer is modified by
this change.
