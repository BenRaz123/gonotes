package main

import (
	"bytes"
	_ "embed"
	"html/template"
)

type pageData struct {
	CurrentFile      string
	CurrentDir       path
	IsDir            bool
	RenderedMarkdown template.HTML
	Files            []string
	Dirs             []string
}

type breadCrumb struct{ Name, Link string }

type path []breadCrumb

func pathFromBuf(p []string) path {
	if len(p) == 0 {
		return []breadCrumb{}
	}
	var buf path
	finalPath := "/"
	for _, part := range p {
		finalPath += part + "/"
		buf = append(buf, breadCrumb{Name: part, Link: finalPath})
	}
	return buf
}

func (p path) String() string {
	if len(p) == 0 {
		return "/"
	}
	return p[len(p)-1].Link
}

//go:embed template.html
var tmpl string

func newPageDataForDir(currentDir []string, files, dirs []string) pageData {
	return pageData{
		CurrentDir: pathFromBuf(currentDir),

		IsDir: true,
		Files: files,
		Dirs:  dirs,
	}
}

func newPageDataForFile(currentDir []string, currentFile, renderedMarkdown string, files, dirs []string) pageData {
	return pageData{
		CurrentDir:       pathFromBuf(currentDir),
		CurrentFile:      currentFile,
		RenderedMarkdown: template.HTML(renderedMarkdown),

		IsDir: false,
		Files: files,
		Dirs:  dirs,
	}
}

func (d pageData) Render() ([]byte, error) {
	t, err := template.New("page").Funcs(template.FuncMap{"add": func(a, b int) int { return a + b }, "minus": func(a, b int) int { return a - b }}).Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	t.Execute(&buf, d)
	return buf.Bytes(), nil
}
