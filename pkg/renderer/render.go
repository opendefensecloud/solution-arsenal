package renderer

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

type renderer struct {
	templateFS    fs.FS
	templateFiles []string
	data          any
	outputName    string
}

func newRenderer() *renderer {
	return &renderer{}
}

func (r *renderer) withTemplateFS(fs embed.FS) *renderer {
	r.templateFS = fs
	return r
}

func (r *renderer) withTemplateFiles(files []string) *renderer {
	r.templateFiles = files
	return r
}

func (r *renderer) withTemplateData(data any) *renderer {
	r.data = data
	return r
}

func (r *renderer) withOutputName(name string) *renderer {
	r.outputName = name
	return r
}

func (r *renderer) render() (*RenderResult, error) {
	tmp, err := os.MkdirTemp("", r.outputName)
	if err != nil {
		return nil, err
	}

	for _, fname := range r.templateFiles {
		tpl, err := template.New(filepath.Base(fname)).Delims("<<", ">>").Funcs(funcMap()).ParseFS(r.templateFS, fname)
		if err != nil {
			_ = os.RemoveAll(tmp)
			return nil, err
		}

		outputPath := filepath.Join(tmp, filepath.Base(fname))
		// Handle nested paths for templates directory
		if filepath.Base(filepath.Dir(fname)) == "templates" {
			// Create templates directory if needed
			templatesDir := filepath.Join(tmp, "templates")
			_ = os.MkdirAll(templatesDir, 0755)
			outputPath = filepath.Join(templatesDir, filepath.Base(fname))
		}

		f, err := os.Create(outputPath)
		if err != nil {
			_ = f.Close()
			_ = os.RemoveAll(tmp)
			return nil, err
		}

		err = tpl.Execute(f, &r.data)
		if err != nil {
			_ = f.Close()
			_ = os.RemoveAll(tmp)
			return nil, err
		}

		_ = f.Close()
	}

	return &RenderResult{
		Dir: tmp,
	}, nil
}
