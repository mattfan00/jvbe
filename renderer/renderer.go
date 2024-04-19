package renderer

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sync"

	"github.com/mattfan00/jvbe/logger"
	"github.com/mattfan00/jvbe/renderer/templates"
)

type Renderer struct {
	templatesCache sync.Map
	log            logger.Logger
}

func New(log logger.Logger) *Renderer {
	return &Renderer{
		log: log,
	}
}

func (r *Renderer) parse(key string, files []string) (*template.Template, error) {
	load, ok := r.templatesCache.Load(key)
	if !ok {
		r.log.Printf("initializing %s", key)
		parsed, err := templates.Parse(files...)
		if err != nil {
			return nil, err
		}

		for _, t := range parsed.Templates() {
			fmt.Println(t.Name())
		}

		r.templatesCache.Store(key, parsed)
		load = parsed
		r.log.Printf("parsed and stored %s", key)
	}

	t, ok := load.(*template.Template)
	if !ok {
		return nil, errors.New("unable to cast to template")
	}

	return t, nil
}

func (r *Renderer) renderPage(w http.ResponseWriter, pageFile string, data any) {
	files := []string{"base.html", "header.html"}
	files = append(files, pageFile)
	t, err := r.parse(pageFile, files)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.ExecuteTemplate(w, "base.html", data)
}

func (r *Renderer) RenderHomePage(w http.ResponseWriter, data any) {
	r.renderPage(w, "pages/home.html", data)
}
