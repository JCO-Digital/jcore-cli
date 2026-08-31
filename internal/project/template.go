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
	RemoteDomain string
	RemoteHost   string
	RemotePath   string
	XdebugMode   string
}

// CurrentTemplateData builds TemplateData from the currently loaded configuration.
func CurrentTemplateData() TemplateData {
	xdebugMode := "off"
	if viper.GetBool("debug") {
		xdebugMode = "develop,debug"
	}

	return TemplateData{
		ProjectName:  viper.GetString("projectName"),
		Theme:        viper.GetString("theme"),
		RemoteDomain: viper.GetString("remoteDomain"),
		RemoteHost:   viper.GetString("remoteHost"),
		RemotePath:   viper.GetString("remotePath"),
		XdebugMode:   xdebugMode,
	}
}

// renderTemplate executes a Go template's content against data.
func renderTemplate(name string, content []byte, data TemplateData) ([]byte, error) {
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
