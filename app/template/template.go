package template

import (
	"embed"
	"errors"
	"html/template"
	"sync"
	"time"

	"github.com/mattfan00/jvbe/logger"
)

//go:embed *
var templatesFs embed.FS

type Manager struct {
	cache sync.Map
	log   logger.Logger
}

func NewManager(log logger.Logger) *Manager {
	return &Manager{
		log: log,
	}
}

// The returned template is named after the first file provided in the args.
// This is so that you can call Execute directly on the template.
func Parse(files ...string) (*template.Template, error) {
	if len(files) == 0 {
		return nil, errors.New("no files provided to parse")
	}

	t := template.New(files[0])

	t.Funcs(template.FuncMap{
		"jsTime":   jsTime,
		"l":        l,
		"add":      add,
		"unescape": unescape,
	})

	t, err := t.ParseFS(templatesFs, files...)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (m *Manager) Parse(key string, files []string) (*template.Template, error) {
	load, ok := m.cache.Load(key)
	if !ok {
		m.log.Printf("initializing %s", key)
		parsed, err := Parse(files...)
		if err != nil {
			return nil, err
		}

		m.cache.Store(key, parsed)
		load = parsed
		m.log.Printf("parsed and stored %s", key)
	}

	t, ok := load.(*template.Template)
	if !ok {
		return nil, errors.New("unable to cast to template")
	}

	return t, nil
}

func jsTime(t time.Time) string {
	return t.Format("2006-01-02T15:04:05Z")
}

func l(i int) []int {
	r := []int{}
	for j := 0; j < i; j++ {
		r = append(r, j+1)
	}

	return r
}

func add(x int, y int) int {
	return x + y
}

func unescape(s string) template.HTML {
	return template.HTML(s)
}
