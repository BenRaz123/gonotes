package main

import (
	"errors"
	"fmt"
	"github.com/benraz123/gonotes/page"
	"log"
	"net/http"
	"os"
	pathpkg "path"
	"strings"

	"github.com/gomarkdown/markdown"
)

type server struct {
	root string
}

func (s server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// cleaned so "/a/" and "/a" are treated the same
	urlPath := pathpkg.Clean(r.URL.Path)
	path := pathpkg.Clean(s.root + "/" + r.URL.Path)

	writebuf := func(b []byte) {
		if _, err := w.Write(b); err != nil {
			log.Fatal(err)
		}
	}

	write := func(s string) {
		writebuf([]byte(s))
	}

	if nexists(path) {
		w.WriteHeader(http.StatusNotFound)
		write(fmt.Sprintf("Could not find file %q\n", r.URL.Path))
		return
	}

	isDir, err := isDir(path)

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		write(fmt.Sprintf("Error statting file %q: %s\n", r.URL.Path, err))
		return
	}

	var parentPath string

	if isDir {
		parentPath = path
	} else {
		parentPath = pathpkg.Dir(path)
	}

	entries, readdirErr := os.ReadDir(parentPath)
	if readdirErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		write(fmt.Sprintf("Error: could not get contents of requested directory: %s", readdirErr))
		return
	}

	var files, dirs []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			dirs = append(dirs, name)
		} else {
			if pathpkg.Ext(name) == ".md" {
				files = append(files, name)
			}
		}
	}

	var leadingDir []string
	if pathpkg.Base(r.URL.Path) != "/" {
		if isDir {
			leadingDir = strings.Split(urlPath, "/")[1:]
		} else {
			if pathpkg.Dir(urlPath) != "/" {
				leadingDir = strings.Split(pathpkg.Dir(urlPath), "/")[1:]
			}
		}
	}

	var out []byte
	var pageErr error

	if isDir {
		out, pageErr = page.Dir(leadingDir, files, dirs)
	} else {
		md, err := os.ReadFile(path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Error: could not read requested file: %s", err)))
		}
		html := markdown.ToHTML(md, nil, nil)
		out, pageErr = page.File(leadingDir, pathpkg.Base(path), string(html), files, dirs)
	}

	if pageErr != nil {
		log.Fatalf("error executing template: %s", err)
	}

	writebuf(out)
}

func isDir(path string) (bool, error) {
	finfo, err := os.Stat(path)
	return finfo.IsDir(), err
}

func nexists(path string) bool {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return true
	}
	return false
}

func main() {
	http.Handle("/", server{"/notes"})
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
