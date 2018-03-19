// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// github.com/aah-cb/ve-pug source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package pug

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"

	"aahframework.org/log.v0"
)

// tmplInclude method renders given template with View Args and imports into
// current template.
func tmplInclude(name string, viewArgs map[string]interface{}) template.HTML {
	if !strings.HasPrefix(name, "common") {
		name = "common/" + name
	}
	name = filepath.ToSlash(name)

	tmpl := commonTemplates.Lookup(name)
	if tmpl == nil {
		log.Warnf("pugviewengine: common template not found: %s", name)
		return template.HTML("")
	}

	buf := acquireBuffer()
	defer releaseBuffer(buf)
	if err := tmpl.Execute(buf, viewArgs); err != nil {
		log.Error(err)
		return template.HTML("")
	}

	return template.HTML(buf.String())
}

func acquireBuffer() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func releaseBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}
