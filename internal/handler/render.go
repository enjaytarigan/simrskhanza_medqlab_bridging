package handler

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

//go:embed templates/*
var templateFS embed.FS

var baseTpl *template.Template

func formatMoney(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	n := int64(v + 0.5)
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func initTemplates() error {
	t, err := template.New("").Funcs(template.FuncMap{
		"formatMoney": formatMoney,
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return err
	}
	baseTpl = t
	return nil
}

func render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := baseTpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
