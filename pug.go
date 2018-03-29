// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// github.com/aah-cb/ve-pug source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package pug

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"

	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
	"aahframework.org/view.v0"

	puglib "github.com/Joker/jade"
)

const noLayout = "nolayout"

var (
	commonTemplates *view.Templates
	bufPool         *sync.Pool
)

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// type PugViewEngine and its method
//___________________________________

// PugViewEngine formerly know as Jade.
type PugViewEngine struct {
	*view.EngineBase
}

// Init method initializes Pug (jade) view engine with given aah application config
// and application views base directory.
func (e *PugViewEngine) Init(appCfg *config.Config, baseDir string) error {
	if e.EngineBase == nil {
		e.EngineBase = &view.EngineBase{}
	}

	if err := e.EngineBase.Init(appCfg, baseDir, "pug", ".pug"); err != nil {
		return err
	}

	// Add template funcs
	view.AddTemplateFunc(template.FuncMap{
		"include": tmplInclude,
		"import":  tmplInclude, // alias to include
	})

	// load common templates
	if err := e.loadCommonTemplates(); err != nil {
		return err
	}

	// collect all layouts
	layouts, err := e.LayoutFiles()
	if err != nil {
		return err
	}

	// load pages templates
	if err = e.loadLayoutTemplates(layouts); err != nil {
		return err
	}

	// load no layout pages templates, if enabled
	if !e.IsLayoutEnabled {
		// since pages directory processed above, no error expected here
		_ = e.loadNonLayoutTemplates("pages")
	}

	// load errors templates
	if ess.IsFileExists(filepath.Join(e.BaseDir, "errors")) {
		if err = e.loadNonLayoutTemplates("errors"); err != nil {
			return err
		}
	}

	return nil
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// type PugViewEngine, unexported methods
//________________________________________

func (e *PugViewEngine) loadCommonTemplates() error {
	commons, err := e.FilesPath("common")
	if err != nil {
		return err
	}

	puglib.LeftDelim = e.LeftDelim
	puglib.RightDelim = e.RightDelim
	commonTemplates = &view.Templates{}
	bufPool = &sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}
	prefix := filepath.Dir(e.BaseDir)
	for _, file := range commons {
		if !strings.HasSuffix(file, e.FileExt) {
			log.Errorf("pugviewengine: not a valid template extension[%s]: %s", e.FileExt, view.TrimPathPrefix(prefix, file))
			continue
		}

		log.Tracef("Parsing file: %s", view.TrimPathPrefix(prefix, file))
		tstr, err := puglib.ParseFile(file)
		if err != nil {
			return err
		}

		tmplKey := view.StripPathPrefixAt(filepath.ToSlash(file), "views/")
		tmpl := e.NewTemplate(tmplKey)

		tstr = e.AntiCSRFField.InsertOnString(tstr)
		if tmpl, err = tmpl.Parse(tstr); err != nil {
			return err
		}

		if err = commonTemplates.Add(tmplKey, tmpl); err != nil {
			return err
		}
	}

	return nil
}

func (e *PugViewEngine) loadLayoutTemplates(layouts []string) error {
	dirs, err := e.DirsPath("pages")
	if err != nil {
		return err
	}

	puglib.LeftDelim = e.LeftDelim
	puglib.RightDelim = e.RightDelim

	// Temp directory
	tmpDir, _ := ioutil.TempDir("", "pug_layout_pages")

	prefix := filepath.Dir(e.BaseDir)
	var errs []error
	for _, layout := range layouts {
		layoutKey := strings.ToLower(filepath.Base(layout))

		layoutStr, err := puglib.ParseFile(layout)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		lfilePath := filepath.Join(tmpDir, view.StripPathPrefixAt(layout, "views"))
		e.writeFile(lfilePath, layoutStr)

		for _, dir := range dirs {
			files, err := filepath.Glob(filepath.Join(dir, "*"+e.FileExt))
			if err != nil {
				errs = append(errs, err)
				continue
			}

			for _, file := range files {
				log.Tracef("Parsing files: %s", view.TrimPathPrefix(prefix, []string{file, layout}...))
				tstr, err := puglib.ParseFile(file)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				tfilePath := filepath.Join(tmpDir, "views", view.StripPathPrefixAt(file, "views"))
				e.writeFile(tfilePath, tstr)

				tfiles := []string{tfilePath, lfilePath}
				tmplKey := view.StripPathPrefixAt(filepath.ToSlash(file), "views/")
				tmpl := e.NewTemplate(tmplKey)
				tmplfiles := e.AntiCSRFField.InsertOnFiles(tfiles...)

				if tmpl, err = tmpl.ParseFiles(tmplfiles...); err != nil {
					errs = append(errs, err)
					continue
				}

				if err = e.AddTemplate(layoutKey, tmplKey, tmpl); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	return e.ParseErrors(errs)
}

func (e *PugViewEngine) loadNonLayoutTemplates(scope string) error {
	dirs, err := e.DirsPath(scope)
	if err != nil {
		return err
	}

	puglib.LeftDelim = e.LeftDelim
	puglib.RightDelim = e.RightDelim
	prefix := filepath.Dir(e.BaseDir)
	var errs []error
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*"+e.FileExt))
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, file := range files {
			tstr, err := puglib.ParseFile(file)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			tmplKey := noLayout + "-" + view.StripPathPrefixAt(filepath.ToSlash(file), "views/")
			tmpl := e.NewTemplate(tmplKey)
			tstr = e.AntiCSRFField.InsertOnString(tstr)

			log.Tracef("Parsing file: %s", view.TrimPathPrefix(prefix, file))
			if tmpl, err = tmpl.Parse(tstr); err != nil {
				errs = append(errs, err)
				continue
			}

			if err = e.AddTemplate(noLayout, tmplKey, tmpl); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return e.ParseErrors(errs)
}

func (e *PugViewEngine) writeFile(file, content string) {
	_ = ess.MkDirAll(filepath.Dir(file), 0755)
	_ = ioutil.WriteFile(file, []byte(content), 0755)
}

func init() {
	// Register pug view engine
	if err := view.AddEngine("pug", &PugViewEngine{}); err != nil {
		log.Fatal(err)
	}
}
