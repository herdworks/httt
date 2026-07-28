# HyperTemplates

The `herdworks/hypertemplates-go` package is the HyperTemplates Core templating engine.

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

Use one or more `ht-with` modifiers to give complete `${ ... }` lookups short,
element-local names. The exact syntax is `${ variable } as binding`; bindings
run in source order, so a later one can reference an earlier name:

```html
<example-fragment
  ht-include="fragments/example"
  ht-apply
  ht-with="${ page.foo } as foo"
></example-fragment>
```

With `ht-apply`, bindings apply only to that element's attributes and direct
text-node children. With `ht-each`, they run after the loop item and `each.*`
metadata are available, and remain available to that clone and its children.
An `ht-with` not consumed by either directive causes an implicit `ht-apply` on
the same element. Bindings use a shallow local data scope and never mutate the
outer render data. `each` remains reserved and cannot be used as a binding.

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
`ht-param`, `ht-content`, `ht-query`, and `ht-pipe` declarations. The command
can write a complete migrated copy with `--output <directory>` and uses
interactive review with `--interactive`; unsupported or ambiguous declarations
are left unchanged and reported with their locations.

The removed runtime directives above fail with a migration hint. `ht-attrs`
remains active: each repeated value must be one complete `${ ... }` expression
that resolves to a string-keyed attribute map, and later maps win conflicts.
Use `ht-apply` for ordinary templated attributes and text. `ht-base` requires
repeatable `target on ${ value }` declarations, while `ht-pipe` requires a
quoted literal destination such as `to "style#components" as css`.
