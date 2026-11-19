package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A small copy of nice/test/arch for one service: file names carry their layer, and imports run one way.

func suffixesFor(rel string) []string {
	parts := strings.Split(rel, "/")
	for _, p := range parts[:len(parts)-1] {
		if p == "internal" {
			return []string{"_helper.go", "_mapper.go"}
		}
	}
	switch {
	case parts[0] == "domain":
		return []string{"_domain.go"}
	case parts[0] == "app" && len(parts) == 4 && parts[2] == "usecase":
		return []string{"_usecase.go", "_contract.go"}
	case parts[0] == "app" && len(parts) == 4 && parts[2] == "dto":
		return []string{"_dto.go"}
	case parts[0] == "adapter" && parts[1] == "cli":
		return []string{"_handler.go", "_routes.go", "routes.go"}
	case parts[0] == "adapter" && parts[1] == "nadi":
		return []string{"_gateway.go"}
	}
	return nil
}

func walk(t *testing.T, fn func(rel string, f *ast.File)) {
	t.Helper()
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			return err
		}
		fn(filepath.ToSlash(p), f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFileNamesCarryTheirLayer(t *testing.T) {
	walk(t, func(rel string, _ *ast.File) {
		want := suffixesFor(rel)
		for _, s := range want {
			if strings.HasSuffix(rel, s) {
				return
			}
		}
		if want != nil {
			t.Errorf("%s: file name must end with one of %v", rel, want)
		}
	})
}

func normalize(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "")) }

// A <name>_usecase.go holds exactly one exported function named after the file; service_<resource>_usecase.go
// declares Service.
func TestUseCaseFileNamesMatchTheirFunction(t *testing.T) {
	walk(t, func(rel string, f *ast.File) {
		parts := strings.Split(rel, "/")
		if len(parts) != 4 || parts[0] != "app" || parts[2] != "usecase" || !strings.HasSuffix(rel, "_usecase.go") {
			return
		}
		base := strings.TrimSuffix(filepath.Base(rel), "_usecase.go")
		var exported []string
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					exported = append(exported, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok && ts.Name.Name == "Service" {
						exported = append(exported, "Service")
					}
				}
			}
		}
		if normalize(base) == normalize("service_"+parts[1]) {
			if len(exported) != 1 || exported[0] != "Service" {
				t.Errorf("%s: a service_ file must declare type Service and nothing else exported, has %v", rel, exported)
			}
			return
		}
		if len(exported) != 1 || normalize(exported[0]) != normalize(base) {
			t.Errorf("%s: must hold exactly one exported function named after the file (%s), has %v", rel, base, exported)
		}
	})
}

// forbidden lists, per layer folder, the import prefixes that layer may not use.
var forbidden = []struct {
	layer string
	deny  []string
}{
	{"domain/", []string{"net/http", "github.com/nice-platform/nice/", "/internal/app", "/internal/adapter", "/internal/platform"}},
	{"app/", []string{"net/http", "github.com/nice-platform/nice/", "/internal/adapter", "/internal/platform", "/internal/bootstrap"}},
	{"adapter/", []string{"/internal/bootstrap"}},
}

func TestImportsRunOneWay(t *testing.T) {
	walk(t, func(rel string, f *ast.File) {
		for _, is := range f.Imports {
			path, _ := strconv.Unquote(is.Path.Value)
			for _, rule := range forbidden {
				if !strings.HasPrefix(rel, rule.layer) {
					continue
				}
				for _, d := range rule.deny {
					if path == d || strings.Contains(path, d) {
						t.Errorf("%s imports %s: %s must not depend on it", rel, path, rule.layer)
					}
				}
			}
			if strings.HasPrefix(rel, "app/") && strings.Contains(rel, "/dto/") && strings.HasSuffix(path, "/usecase") {
				t.Errorf("%s imports %s: a dto must not depend on a use case", rel, path)
			}
		}
	})
}
