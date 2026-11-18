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

const module = "github.com/nice-platform/example/sharedservice-oauth/backend/internal/"

// forbidden lists, per layer folder, the import prefixes that layer may not use.
var forbidden = map[string][]string{
	"domain/":         {module + "app", module + "adapter", module + "bootstrap", module + "platform", "net/http"},
	"app/":            {module + "adapter", module + "bootstrap", "net/http"},
	"app/device/dto":  {module + "app/device/usecase"},
	"app/session/dto": {module + "app/session/usecase"},
	"adapter/":        {module + "bootstrap"},
	"platform/":       {module + "app", module + "adapter", module + "domain", module + "bootstrap"},
}

// suffixes lists, per folder, the file name endings allowed for a non-test file.
var suffixes = []struct {
	prefix string
	ends   []string
}{
	{"domain/", []string{"_domain.go"}},
	{"app/device/usecase/", []string{"_usecase.go", "_contract.go"}},
	{"app/device/dto/", []string{"_dto.go"}},
	{"app/session/usecase/", []string{"_usecase.go", "_contract.go"}},
	{"app/session/dto/", []string{"_dto.go"}},
	{"adapter/memory/session/", []string{"_repository.go"}},
	{"adapter/nice/", []string{"_gateway.go"}},
	{"adapter/rest/auth/dto/", []string{"_dto.go"}},
	{"adapter/rest/auth/", []string{"_handler.go", "_routes.go", "_middleware.go", "_helper.go"}},
	{"adapter/rest/web/", []string{"_handler.go", "_routes.go"}},
	{"adapter/memory/device/", []string{"_repository.go"}},
	{"adapter/postgres/device/", []string{"_repository.go"}},
	{"adapter/rest/dto/", []string{"_dto.go"}},
	{"adapter/rest/device/dto/", []string{"_dto.go"}},
	{"adapter/rest/device/internal/", []string{"_helper.go", "_mapper.go"}},
	{"adapter/rest/internal/", []string{"_helper.go", "_mapper.go"}},
	{"adapter/rest/device/", []string{"_handler.go", "_routes.go"}},
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
