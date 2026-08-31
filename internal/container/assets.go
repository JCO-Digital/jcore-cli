package container

import (
	"embed"
)

// BaseAssets holds the project skeleton files
//
//go:embed all:base
var BaseAssets embed.FS

// TemplateAssets holds the project templates
//
//go:embed all:templates
var TemplateAssets embed.FS
