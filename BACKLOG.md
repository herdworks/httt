# Backlog

## File Path Safety

Track follow-up hardening for file discovery and root-scoped path resolution.

* Use `filepath.IsLocal` as an explicit lexical guard anywhere we accept caller-provided relative paths. This should reject absolute paths, empty paths, and parent traversal before the value is joined to a trusted root.
* Keep using `filepath.Localize` when converting slash-separated paths into OS-local paths. Treat localization failures as invalid input, not as missing files.
* Add symlink escape protection for reads and copies that operate under a trusted root:
  * Decide whether discovery should skip symlinks entirely, reject them with an error, or allow only symlinks that resolve inside the same root.
  * For allowed symlinks, compare `filepath.EvalSymlinks(root)` and `filepath.EvalSymlinks(root + relpath)` and reject resolved paths outside the resolved root.
  * Add regression tests for symlinks pointing outside the website, theme, and content roots.
* Centralize all source-file opens through root + relative path helpers so callers do not manually join or normalize paths.
* Review archive generation and package creation for the same root escape protections before accepting untrusted themes.
