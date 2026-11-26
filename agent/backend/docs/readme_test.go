package docs_test

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func section(text, heading string) string {
	start := strings.Index(text, "\n"+heading+"\n")
	if start < 0 {
		return ""
	}
	rest := text[start+len(heading)+2:]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return rest[:end]
	}
	return rest
}

func TestReadmesDocumentTheVariablesOfEnvExample(t *testing.T) {
	var want []string
	for _, m := range regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)=`).FindAllStringSubmatch(readFile(t, "../.env.example"), -1) {
		want = append(want, m[1])
	}
	slices.Sort(want)
	for file, heading := range map[string]string{"README.md": "## Konfigurasi", "README.en.md": "## Configuration"} {
		var got []string
		for _, m := range regexp.MustCompile("(?m)^\\| `([A-Z][A-Z0-9_]*)` \\|").FindAllStringSubmatch(section(readFile(t, "../"+file), heading), -1) {
			got = append(got, m[1])
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s documents %v, .env.example has %v", file, got, want)
		}
	}
}

func TestChecklistIsNumberedAndSmokePointsAtItsSteps(t *testing.T) {
	steps := map[string][]int{}
	for _, file := range []string{"README.md", "README.en.md"} {
		for _, m := range regexp.MustCompile(`(?m)^### (\d+)\. `).FindAllStringSubmatch(readFile(t, "../"+file), -1) {
			n, _ := strconv.Atoi(m[1])
			steps[file] = append(steps[file], n)
		}
		for i, n := range steps[file] {
			if n != i+1 {
				t.Fatalf("%s numbers the checklist %v", file, steps[file])
			}
		}
	}
	if !slices.Equal(steps["README.md"], steps["README.en.md"]) {
		t.Fatalf("the checklists differ: %v and %v", steps["README.md"], steps["README.en.md"])
	}
	for _, m := range regexp.MustCompile(`langkah (\d+)`).FindAllStringSubmatch(readFile(t, "../scripts/smoke.sh"), -1) {
		if n, _ := strconv.Atoi(m[1]); n < 1 || n > len(steps["README.md"]) {
			t.Errorf("scripts/smoke.sh points at step %d, which the checklist does not have", n)
		}
	}
}
