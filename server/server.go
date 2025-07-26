package server

import (
	"errors"
	"fmt"
	htmlpkg "html"
	"log"
	"net/http"
	"os"
	pathpkg "path"
	"strings"

	"github.com/benraz123/gonotes/page"
	"github.com/gomarkdown/markdown"
)

func New(showAll bool, roots ...string) http.Handler {
	return server{showAll: showAll, roots: roots}
}

type server struct {
	showAll bool
	roots   []string
}

func rootInfo(root, path string) (has bool, isDir bool, err error) {
	path = pathpkg.Clean(path)
	info, err := os.Stat(root + "/" + path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil && strings.HasSuffix(err.Error(), "not a directory") {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return true, info.IsDir(), nil
}

func (s server) getCurrentFolder(urlPath string, isDir bool) []string {
	var parentPath string = urlPath
	if !isDir {
		parentPath = pathpkg.Dir(urlPath)
	}

	if parentPath == "/" {
		return []string{}
	}

	return strings.Split(pathpkg.Clean(parentPath), "/")[1:]
}

type lsError struct {
	dirName string
	err     error
}

func (l lsError) Error() string {
	return fmt.Sprintf("error listing dir %q: %s", l.dirName, l.err)
}

func (s server) getFilesAndDirs(resolvedPaths []string, urlPath string, isDir bool) (files, dirs []string, err error) {
	ls := func(dir string) (fileS, dirS []string, err error) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, nil, lsError{dir, err}
		}
		for _, de := range entries {
			if de.IsDir() {
				dirS = append(dirS, de.Name())
			} else {
				fileS = append(fileS, de.Name())
			}
		}
		if !s.showAll {
			fileS = filter(fileS, func(p string) bool { return pathpkg.Ext(p) == ".md" })
		}
		return
	}

	if !isDir {
		parent := pathpkg.Dir(urlPath)
		resolvedPaths, _, _, _ := s.resolve(parent)
		return s.getFilesAndDirs(resolvedPaths, parent, true)
	}

	if len(resolvedPaths) == 0 {
		return ls(resolvedPaths[0])
	}

	// they are all dirs at this point
	for _, path := range resolvedPaths {
		fileS, dirS, lsErr := ls(path)
		files = append(files, fileS...)

		dirs = append(dirs, dirS...)
		if lsErr != nil {
			return nil, nil, lsErr
		}
	}

	return
}

func filter[T any](x []T, f func(T) bool) []T {
	var ret []T
	for _, xs := range x {
		if f(xs) {
			ret = append(ret, xs)
		}
	}
	return ret
}

// If we have `server{[]string{"/a", "/b"}}` and the path is "/hello.md", "/a/hello.md" is returned if that file exists and if not "/b/hello.md" is returned and if that doesn't exist, no path is returned
// path passed to this should not be cleaned; we want to preserve the optional '/' at the end.
func (s server) resolve(path string) (resolvedPaths []string, exists bool, isDir bool, err error) {
	type completion struct {
		path  string
		isDir bool
	}

	var hasDir, hasFile bool
	var completions []completion

	for _, root := range s.roots {
		has, isDir, err := rootInfo(root, path)
		if err != nil {
			return nil, false, false, err
		}
		if has {
			if isDir {
				hasDir = true
			} else {
				hasFile = true
			}
			completions = append(completions, completion{path: pathpkg.Clean(root + "/" + path), isDir: isDir})
		}
	}

	if len(completions) == 0 {
		return nil, false, false, nil
	}

	if !hasDir || (len(completions) == 1) {
		return []string{completions[0].path}, true, completions[0].isDir, nil
	}

	if !hasFile || strings.HasSuffix(path, "/") {
		for _, completion := range completions {
			if completion.isDir {
				resolvedPaths = append(resolvedPaths, completion.path)
			}
		}
		return resolvedPaths, true, true, nil
	}

	for _, completion := range completions {
		if !completion.isDir {
			return []string{completion.path}, true, false, nil
		}
	}

	// unreachable
	log.Println("'warning: function server %+#v .resolve reached unreachable place: %q", s, path)
	return
}

func (s server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("server: GET %q", r.URL.Path)

	writebuf := func(status int, b []byte) {
		w.WriteHeader(status)
		if status != http.StatusOK {
			log.Printf("server: (returned %d) %s\n", status, b)
		}
		if _, err := w.Write(b); err != nil {
			log.Fatalf("error writing http response to url %q: %s", r.URL.Path, err)
		}
	}
	write := func(status int, format string, args ...any) { writebuf(status, []byte(fmt.Sprintf(format, args...))) }

	resolvedPaths, exists, isDir, err := s.resolve(r.URL.Path)

	if err != nil {
		write(http.StatusInternalServerError, "problem retrieving file information for %q: %s", r.URL.Path, err)
		return
	}

	if !exists {
		write(http.StatusNotFound, "file %q not found", r.URL.Path)
		return
	}

	if !isDir && !s.showAll && pathpkg.Ext(r.URL.Path) != ".md" {
		write(http.StatusBadRequest, "Request for file %q that is not markown. To turn off this filter, run the server with the -a or --all flag", r.URL.Path)
		return
	}

	currentFolder := s.getCurrentFolder(r.URL.Path, isDir)
	files, dirs, err := s.getFilesAndDirs(resolvedPaths, r.URL.Path, isDir)

	if err != nil {
		write(http.StatusInternalServerError, "could not list files in directory: %q: %s", r.URL.Path, err)
		return
	}

	var html []byte
	var htmlErr error

	switch {
	case isDir:
		html, htmlErr = page.Dir(currentFolder, files, dirs)
	default:

		md, err := os.ReadFile(resolvedPaths[0] /* the only path */)
		if err != nil {
			write(http.StatusFailedDependency, "could not read file %q: %s", r.URL.Path, err)
			return
		}

		if pathpkg.Ext(r.URL.Path) != ".md" {
			html, htmlErr = page.File(currentFolder, pathpkg.Base(r.URL.Path), fmt.Sprintf(`<pre style="white-space:pre-wrap">%s</pre>`, htmlpkg.EscapeString(string(md))), files, dirs)
		} else {
			html, htmlErr = page.File(currentFolder, pathpkg.Base(r.URL.Path), string(markdown.ToHTML(md, nil, nil)), files, dirs)
		}
	}

	if htmlErr != nil {
		write(http.StatusNoContent, "failed to create page: %s", htmlErr)
		return
	}

	writebuf(http.StatusOK, html)
}
