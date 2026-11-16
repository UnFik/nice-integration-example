// Package internal holds the architecture test: the dependency and file
// naming rules of the layout, modeled on nice/test/arch.
package internal

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/nice-platform/example/shared-service-oauth/internal/"

// forbidden lists, per layer folder, the import prefixes that layer may not use.
var forbidden = map[string][]string{
	"domain/":      {module + "app", module + "adapter", module + "bootstrap", module + "platform", "net/http"},
	"app/":         {module + "adapter", module + "bootstrap", "net/http"},
	"app/item/dto": {module + "app/item/usecase"},
	"adapter/":     {module + "bootstrap"},
	"platform/":    {module + "app", module + "adapter", module + "domain", module + "bootstrap"},
}

// suffixes lists, per folder, the file name endings allowed for a non-test file.
var suffixes = []struct {
	prefix string
	ends   []string
}{
	{"domain/", []string{"_domain.go"}},
	{"app/item/usecase/", []string{"_usecase.go", "_contract.go"}},
	{"app/item/dto/", []string{"_dto.go"}},
	{"adapter/memory/item/", []string{"_repository.go"}},
	{"adapter/rest/dto/", []string{"_dto.go"}},
	{"adapter/rest/item/dto/", []string{"_dto.go"}},
	{"adapter/rest/item/internal/", []string{"_helper.go", "_mapper.go"}},
	{"adapter/rest/internal/", []string{"_helper.go", "_mapper.go"}},
	{"adapter/rest/item/", []string{"_handler.go", "_routes.go"}},
	{"adapter/rest/system/", []string{"_handler.go", "_routes.go"}},
	{"adapter/rest/", []string{"routes.go"}},
}

func walk(t *testing.T, fn func(rel string)) {
	t.Helper()
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		fn(filepath.ToSlash(p))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestImportRules(t *testing.T) {
	walk(t, func(rel string) {
		f, err := parser.ParseFile(token.NewFileSet(), rel, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for layer, banned := range forbidden {
			if !strings.HasPrefix(rel, layer) {
				continue
			}
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				for _, b := range banned {
					if path == b || strings.HasPrefix(path, b+"/") {
						t.Errorf("%s imports %s, which its layer may not use", rel, path)
					}
				}
			}
		}
	})
}

func TestFileNameSuffixes(t *testing.T) {
	walk(t, func(rel string) {
		for _, s := range suffixes {
			if !strings.HasPrefix(rel, s.prefix) {
				continue
			}
			for _, e := range s.ends {
				if strings.HasSuffix(rel, e) {
					return
				}
			}
			t.Errorf("%s must end in one of %v", rel, s.ends)
			return
		}
	})
}
