// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// github.com/aah-cb/ve-pug source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package pug

import (
	"bytes"
	"errors"
	"fmt"
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
	cfg             *config.Config
	baseDir         string
	layouts         map[string]*view.Templates
	viewFileExt     string
	caseSensitive   bool
	isLayoutEnabled bool
	leftDelim       string
	rightDelim      string
	antiCSRFField   *view.AntiCSRFField
}

// Init method initialize a pug (jade) template engine with given aah application config
// and application views base path.
func (e *PugViewEngine) Init(appCfg *config.Config, baseDir string) error {
	// check base directory
	if !ess.IsFileExists(baseDir) {
		return fmt.Errorf("pugviewengine: views base dir is not exists: %s", baseDir)
	}

	// initialize
	e.baseDir = baseDir
	e.cfg = appCfg
	e.viewFileExt = e.cfg.StringDefault("view.ext", ".pug")
	e.caseSensitive = e.cfg.BoolDefault("view.case_sensitive", false)
	e.isLayoutEnabled = e.cfg.BoolDefault("view.default_layout", true)

	delimiter := strings.Split(e.cfg.StringDefault("view.delimiters", view.DefaultDelimiter), ".")
	if len(delimiter) != 2 || ess.IsStrEmpty(delimiter[0]) || ess.IsStrEmpty(delimiter[1]) {
		return fmt.Errorf("pugviewengine: config 'view.delimiters' value is invalid")
	}
	e.leftDelim, e.rightDelim = delimiter[0], delimiter[1]

	e.layouts = make(map[string]*view.Templates)

	// Anti CSRF, repurpose it from aahframework.org/view.v0
	e.antiCSRFField = view.NewAntiCSRFField("pug", e.leftDelim, e.rightDelim)

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
	layouts, err := e.findLayouts()
	if err != nil {
		return err
	}

	// load pages templates
	if err = e.loadLayoutTemplates(layouts); err != nil {
		return err
	}

	// load no layout pages templates, if enabled
	if !e.isLayoutEnabled {
		// since pages directory processed above, no error expected here
		_ = e.loadNonLayoutTemplates("pages")
	}

	// load errors templates
	if ess.IsFileExists(filepath.Join(e.baseDir, "errors")) {
		if err = e.loadNonLayoutTemplates("errors"); err != nil {
			return err
		}
	}

	return nil
}

// Get method returns the template based given name if found, otherwise nil.
func (e *PugViewEngine) Get(layout, path, tmplName string) (*template.Template, error) {
	if ess.IsStrEmpty(layout) {
		layout = noLayout
	}

	if l, found := e.layouts[layout]; found {
		key := filepath.Join(path, tmplName)
		if layout == noLayout {
			key = noLayout + "-" + key
		}

		if !e.caseSensitive {
			key = strings.ToLower(key)
		}

		if t := l.Lookup(key); t != nil {
			return t, nil
		}
	}

	return nil, view.ErrTemplateNotFound
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// type PugViewEngine, unexported methods
//________________________________________

func (e *PugViewEngine) loadCommonTemplates() error {
	baseDir := filepath.Join(e.baseDir, "common")
	if !ess.IsFileExists(baseDir) {
		return fmt.Errorf("pugviewengine: common base dir is not exists: %s", baseDir)
	}

	puglib.LeftDelim = e.leftDelim
	puglib.RightDelim = e.rightDelim

	commons, err := ess.FilesPath(baseDir, true)
	if err != nil {
		return err
	}

	commonTemplates = &view.Templates{}
	bufPool = &sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}
	prefix := filepath.Dir(e.baseDir)
	for _, file := range commons {
		if !strings.HasSuffix(file, e.viewFileExt) {
			log.Errorf("pugviewengine: not a valid template extension[%s]: %s", e.viewFileExt, view.TrimPathPrefix(prefix, file))
			continue
		}

		tstr, err := puglib.ParseFile(file)
		if err != nil {
			return err
		}

		tmplKey := view.StripPathPrefixAt(filepath.ToSlash(file), "views/")
		tmpl := template.New(tmplKey).Funcs(view.TemplateFuncMap).Delims(e.leftDelim, e.rightDelim)

		tstr = e.antiCSRFField.InsertOnString(tstr)
		if tmpl, err = tmpl.Parse(tstr); err != nil {
			return err
		}

		if err = commonTemplates.Add(tmplKey, tmpl); err != nil {
			return err
		}
	}

	return nil
}

func (e *PugViewEngine) findLayouts() ([]string, error) {
	baseDir := filepath.Join(e.baseDir, "layouts")
	if !ess.IsFileExists(baseDir) {
		return nil, fmt.Errorf("pugviewengine: layouts base dir is not exists: %s", baseDir)
	}

	return filepath.Glob(filepath.Join(baseDir, "*"+e.viewFileExt))
}

func (e *PugViewEngine) loadLayoutTemplates(layouts []string) error {
	baseDir := filepath.Join(e.baseDir, "pages")
	if !ess.IsFileExists(baseDir) {
		return fmt.Errorf("pugviewengine: pages base dir is not exists: %s", baseDir)
	}

	dirs, err := ess.DirsPath(baseDir, true)
	if err != nil {
		return err
	}

	puglib.LeftDelim = e.leftDelim
	puglib.RightDelim = e.rightDelim

	// Temp directory
	tmpDir, _ := ioutil.TempDir("", "pug_layout_pages")

	prefix := filepath.Dir(e.baseDir)
	var errs []error
	for _, layout := range layouts {
		layoutKey := strings.ToLower(filepath.Base(layout))
		if e.layouts[layoutKey] == nil {
			e.layouts[layoutKey] = &view.Templates{}
		}

		layoutStr, err := puglib.ParseFile(layout)
		if err != nil {
			return err
		}

		lfilePath := filepath.Join(tmpDir, view.StripPathPrefixAt(layout, "views"))
		e.writeFile(lfilePath, layoutStr)

		for _, dir := range dirs {
			files, err := filepath.Glob(filepath.Join(dir, "*"+e.viewFileExt))
			if err != nil {
				errs = append(errs, err)
				continue
			}

			for _, file := range files {
				tstr, err := puglib.ParseFile(file)
				if err != nil {
					return err
				}

				tfilePath := filepath.Join(tmpDir, "views", view.StripPathPrefixAt(file, "views"))
				e.writeFile(tfilePath, tstr)

				tfiles := []string{tfilePath, lfilePath}
				tmplKey := view.StripPathPrefixAt(filepath.ToSlash(file), "views/")
				tmpl := template.New(tmplKey).Funcs(view.TemplateFuncMap).Delims(e.leftDelim, e.rightDelim)
				tmplfiles := e.antiCSRFField.InsertOnFiles(tfiles...)

				log.Tracef("Parsing files: %s", view.TrimPathPrefix(prefix, []string{file, layout}...))
				if tmpl, err = tmpl.ParseFiles(tmplfiles...); err != nil {
					errs = append(errs, err)
					continue
				}

				if err = e.layouts[layoutKey].Add(tmplKey, tmpl); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	return handleParseError(errs)
}

func (e *PugViewEngine) loadNonLayoutTemplates(scope string) error {
	baseDir := filepath.Join(e.baseDir, scope)
	if !ess.IsFileExists(baseDir) {
		return fmt.Errorf("pugviewengine: %s base dir is not exists: %s", scope, baseDir)
	}

	dirs, err := ess.DirsPath(baseDir, true)
	if err != nil {
		return err
	}

	puglib.LeftDelim = e.leftDelim
	puglib.RightDelim = e.rightDelim

	if e.layouts[noLayout] == nil {
		e.layouts[noLayout] = &view.Templates{}
	}

	prefix := filepath.Dir(e.baseDir)
	var errs []error
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*"+e.viewFileExt))
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, file := range files {
			tstr, err := puglib.ParseFile(file)
			if err != nil {
				return err
			}

			tmplKey := noLayout + "-" + view.StripPathPrefixAt(filepath.ToSlash(file), "views/")
			tmpl := template.New(tmplKey).Funcs(view.TemplateFuncMap).Delims(e.leftDelim, e.rightDelim)
			tstr = e.antiCSRFField.InsertOnString(tstr)

			log.Tracef("Parsing file: %s", view.TrimPathPrefix(prefix, file))
			if tmpl, err = tmpl.Parse(tstr); err != nil {
				errs = append(errs, err)
				continue
			}

			if err = e.layouts[noLayout].Add(tmplKey, tmpl); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return handleParseError(errs)
}

func (e *PugViewEngine) writeFile(file, content string) {
	_ = ess.MkDirAll(filepath.Dir(file), 0755)
	_ = ioutil.WriteFile(file, []byte(content), 0755)
}

func handleParseError(errs []error) error {
	if len(errs) > 0 {
		var msg []string
		for _, e := range errs {
			msg = append(msg, e.Error())
		}
		log.Errorf("View templates parsing error(s):\n    %s", strings.Join(msg, "\n    "))
		return errors.New("pugviewengine: error processing templates, please check the log")
	}
	return nil
}

func init() {
	// Register pug view engine
	if err := view.AddEngine("pug", &PugViewEngine{}); err != nil {
		log.Fatal(err)
	}
}
