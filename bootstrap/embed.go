// Package bootstrap contains the minimal editable theme scaffold used by httt init.
package bootstrap

import "embed"

//go:embed all:theme/*
var Theme embed.FS
