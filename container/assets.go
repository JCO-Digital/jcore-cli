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

// LohkoTemplates holds the Lohko Gutenberg block templates used by `jcore
// create block` (dynamic/static). Mirrored here from
// github.com/JCO-Digital/jcore-lohko-templates rather than fetched live,
// since they change rarely — unlike the jcore-ilme theme (see
// internal/project/theme.go), which is always fetched fresh from GitHub.
//
//go:embed all:lohko-templates
var LohkoTemplates embed.FS
