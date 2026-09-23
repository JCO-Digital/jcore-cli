package project

import (
	"bytes"
	"text/template"

	"github.com/spf13/viper"
)

// TemplateExt is the suffix that marks an embedded skeleton file as a Go
// template to be rendered at scaffold/update time, rather than copied verbatim.
const TemplateExt = ".tmpl"

// TemplateData holds the values available to project skeleton templates.
type TemplateData struct {
	ProjectName  string
	Theme        string
	Branch       string
	RemoteDomain string
	RemoteHost   string
	RemotePath   string
	XdebugMode   string
	YdinVersion  string
}

// CurrentTemplateData builds TemplateData from the currently loaded configuration.
func CurrentTemplateData() TemplateData {
	xdebugMode := "off"
	if viper.GetBool("debug") {
		xdebugMode = "develop,debug"
	}

	templateName := viper.GetString("template")
	if templateName == "" {
		templateName = "jcore3"
	}

	branch := viper.GetString("branch")
	ydinVersion := ""
	if catalog, err := LoadTemplateCatalog(); err == nil {
		if entry, ok := catalog[templateName]; ok {
			if branch == "" {
				branch = entry.Branch
			}
			ydinVersion = entry.YdinVersion(branch)
		}
	}
	if ydinVersion == "" {
		ydinVersion = "^5"
	}

	return TemplateData{
		ProjectName:  viper.GetString("projectName"),
		Theme:        viper.GetString("theme"),
		Branch:       branch,
		RemoteDomain: viper.GetString("remoteDomain"),
		RemoteHost:   viper.GetString("remoteHost"),
		RemotePath:   viper.GetString("remotePath"),
		XdebugMode:   xdebugMode,
		YdinVersion:  ydinVersion,
	}
}

// renderTemplate executes a Go template's content against data. data may be
// any type text/template accepts (TemplateData for the main project
// scaffold, LohkoBlockData for a Lohko block template, ...).
func renderTemplate(name string, content []byte, data any) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
