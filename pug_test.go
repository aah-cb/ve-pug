// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// github.com/aah-cb/ve-pug source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package pug

import (
	"bytes"
	"errors"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aahframework.org/config.v0"
	"aahframework.org/log.v0"
	"aahframework.org/test.v0/assert"
	"aahframework.org/view.v0"
)

func TestPugViewAppPages(t *testing.T) {
	_ = log.SetLevel("trace")
	cfg, _ := config.ParseString(`view { }`)
	e := loadPugViewEngine(t, cfg, "views")

	data := map[string]interface{}{
		"GreetName": "aah framework",
		"PageName":  "home page",
	}

	tmpl, err := e.Get("master.pug", "pages/app", "index.pug")
	assert.Nil(t, err)
	assert.NotNil(t, tmpl)

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "master.pug", data)
	assert.FailNowOnError(t, err, "")

	htmlStr := buf.String()
	t.Logf("HTML String: %s", htmlStr)
	assert.True(t, strings.Contains(htmlStr, "<title>Pug View Engine - aah Go web framework</title>"))
	assert.True(t, strings.Contains(htmlStr, "aah framework home page"))

	tmpl, err = e.Get("no_master", "pages/app", "index.pug")
	assert.NotNil(t, err)
	assert.Nil(t, tmpl)
}

func TestPugViewUserPages(t *testing.T) {
	_ = log.SetLevel("trace")
	cfg, _ := config.ParseString(`view {
		delimiters = "{{.}}"
	}`)
	e := loadPugViewEngine(t, cfg, "views")

	data := map[string]interface{}{
		"GreetName": "aah framework",
		"PageName":  "user home page",
	}

	e.CaseSensitive = true

	tmpl, err := e.Get("master.pug", "pages/user", "index.pug")
	assert.Nil(t, err)
	assert.NotNil(t, tmpl)

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "master.pug", data)
	assert.FailNowOnError(t, err, "")

	htmlStr := buf.String()
	t.Logf("HTML String: %s", htmlStr)
	assert.True(t, strings.Contains(htmlStr, "<title>Pug View Engine - User Home</title>"))
	assert.True(t, strings.Contains(htmlStr, "aah framework user home page"))
	assert.True(t, strings.Contains(htmlStr, `cdnjs.cloudflare.com/ajax/libs/jquery/2.2.4/jquery.min.js`))

	tmpl, err = e.Get("master.html", "pages/user", "not_exists.oug")
	assert.NotNil(t, err)
	assert.Nil(t, tmpl)
}

func TestPugViewUserPagesNoLayout(t *testing.T) {
	_ = log.SetLevel("trace")
	cfg, _ := config.ParseString(`view {
		delimiters = "{{.}}"
		default_layout = false
	}`)
	e := loadPugViewEngine(t, cfg, "views")

	data := map[string]interface{}{
		"GreetName": "aah framework",
		"PageName":  "user home page",
	}

	tmpl, err := e.Get("", "pages/user", "index-nolayout.pug")
	assert.Nil(t, err)
	assert.NotNil(t, tmpl)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	assert.FailNowOnError(t, err, "")

	htmlStr := buf.String()
	t.Logf("HTML String: %s", htmlStr)
	assert.True(t, strings.Contains(htmlStr, "aah framework user home page - no layout"))
}

func TestPugViewBaseDirNotExists(t *testing.T) {
	viewsDir := filepath.Join(getTestdataPath(), "views1")
	e := &PugViewEngine{}
	cfg, _ := config.ParseString(`view { }`)

	err := e.Init(cfg, viewsDir)
	assert.NotNil(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "pugviewengine: views base dir is not exists:"))
}

func TestPugViewDelimitersError(t *testing.T) {
	viewsDir := filepath.Join(getTestdataPath(), "views")
	e := &PugViewEngine{}
	cfg, _ := config.ParseString(`view {
		delimiters = "%%."
	}`)

	err := e.Init(cfg, viewsDir)
	assert.NotNil(t, err)
	assert.Equal(t, "pugviewengine: config 'view.delimiters' value is invalid", err.Error())
}

func TestPugViewErrors(t *testing.T) {
	_ = log.SetLevel("trace")
	cfg, _ := config.ParseString(`view {
		default_layout = false
	}`)

	// No layout directiry
	viewsDir := filepath.Join(getTestdataPath(), "views-no-layouts-dir")
	e := &PugViewEngine{}
	err := e.Init(cfg, viewsDir)
	assert.NotNil(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "pugviewengine: layouts base dir is not exists:"))

	// No Common directory
	viewsDir = filepath.Join(getTestdataPath(), "views-no-common-dir")
	e = &PugViewEngine{}
	err = e.Init(cfg, viewsDir)
	assert.NotNil(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "pugviewengine: common base dir is not exists:"))

	// No Pages directory
	viewsDir = filepath.Join(getTestdataPath(), "views-no-pages-dir")
	e = &PugViewEngine{}
	err = e.Init(cfg, viewsDir)
	assert.NotNil(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "pugviewengine: pages base dir is not exists:"))

	// handle errors methods
	err = e.ParseErrors([]error{errors.New("error 1"), errors.New("error 2")})
	assert.NotNil(t, err)
	assert.Equal(t, "pugviewengine: error processing templates, please check the log", err.Error())
}

func loadPugViewEngine(t *testing.T, cfg *config.Config, dir string) *PugViewEngine {
	// dummy func for test
	view.AddTemplateFunc(template.FuncMap{
		"anitcsrftoken": func(arg interface{}) string {
			return ""
		},
	})

	viewsDir := filepath.Join(getTestdataPath(), dir)
	e := &PugViewEngine{}

	err := e.Init(cfg, viewsDir)
	assert.FailNowOnError(t, err, "")

	assert.Equal(t, viewsDir, e.BaseDir)
	assert.NotNil(t, e.AppConfig)
	assert.NotNil(t, e.Templates)

	return e
}

func getTestdataPath() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "testdata")
}
