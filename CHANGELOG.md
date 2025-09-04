# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org).

## Unreleased

* Added more comments to `template.go` to document processing logic and outstanding questions/issues
* Changed function and variable names in `template.go` to more consistently distinguish between layouts, fragments, elements, and content;
  * A layout is a valid HTML document
  * A fragment is a partial HTML document containing one or more elements
  * An element is a single HTML element
  * Content is user-generated data consisting of text and/or elements
* Cleaned up OPML CRUD methods complete with tests (!)
* Added RFC822 date parser to OPML package

## v0.17.3 (2025-09-02)

Initial release of the `hypertemplates` package from the standalone `herdworks/hypertemplates-go` repo.

* Completed initial migration from `herdworks/hypertexting-monorepo`
* Adoped `v` prefixed tag naming convention beginning with the `v0.17.3` release

## 0.17.2

See `herdworks/hypertexting-monorepo` for version 0.17.2 and prior changes.
