package templates

import (
	"embed"
	"html/template"
	"time"
)

//go:embed *
var templatesFs embed.FS

func Parse(files ...string) (*template.Template, error) {
	t := template.New("")

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
