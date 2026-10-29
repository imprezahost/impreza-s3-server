// Offline source-selection and isolated implementation checks; not a full server build.
package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func fail(format string, a ...any) { fmt.Fprintf(os.Stderr, "ASSERT: "+format+"\n", a...); os.Exit(1) }
func function(root, file, name string) string {
	b, err := os.ReadFile(filepath.Join(root, "cmd", file))
	if err != nil {
		panic(err)
	}
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, file, b, 0)
	if err != nil {
		panic(err)
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return string(b[fs.Position(fn.Pos()).Offset:fs.Position(fn.End()).Offset])
		}
	}
	fail("missing function %s", name)
	return ""
}

func main() {
	root, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}
	if len(os.Args) > 1 {
		root, err = filepath.Abs(os.Args[1])
		if err != nil {
			panic(err)
		}
	}
	dir := filepath.Join(root, "cmd")
	entries, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	for _, optIn := range []bool{false, true} {
		ctx := build.Default
		ctx.GOOS = "linux"
		ctx.GOARCH = "amd64"
		ctx.BuildTags = []string{"kqueue"}
		if optIn {
			ctx.BuildTags = append(ctx.BuildTags, "minio_console")
		}
		found := 0
		for _, e := range entries {
			n := e.Name()
			if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
				continue
			}
			match, err := ctx.MatchFile(dir, n)
			if err != nil {
				panic(err)
			}
			if !match {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, n), nil, parser.ImportsOnly)
			if err != nil {
				panic(err)
			}
			for _, i := range f.Imports {
				p, err := strconv.Unquote(i.Path.Value)
				if err != nil {
					panic(err)
				}
				if p == "github.com/minio/selfupdate" || strings.HasPrefix(p, "github.com/minio/selfupdate/") {
					fail("selfupdate selected in %s", n)
				}
				if p == "github.com/minio/console" || strings.HasPrefix(p, "github.com/minio/console/") {
					found++
					fail("selected source imports removed Console from %s", n)
				}
			}
		}
		if found != 0 {
			fail("legacy Console remains with old build tag")
		}
		fmt.Printf("PASS: linux/amd64 source selection minio_console=%v Console imports=%d\n", optIn, found)
	}
	// Execute the exact three updater functions in
	// a stdlib-only package. This does not certify their full server integration.
	tmp, err := os.MkdirTemp("", "s3-maintenance-boundaries-")
	if err != nil {
		panic(err)
	}
	source := "package cmd\nimport(\"fmt\";\"io\";\"net/url\")\n"
	for _, n := range []string{"downloadBinary", "verifyBinary", "commitBinary"} {
		source += function(root, "update.go", n) + "\n"
	}
	test := "package cmd\nimport(\"testing\";\"net/url\")\ntype forbiddenUpdateReader struct{}\nfunc(forbiddenUpdateReader)Read([]byte)(int,error){panic(\"unexpected executable read\")}\n"
	for _, n := range []string{"TestInPlaceUpdateMechanicsDisabled"} {
		test += function(root, "update_disabled_test.go", n) + "\n"
	}
	if err = os.WriteFile(filepath.Join(tmp, "boundary.go"), []byte(source), 0o600); err != nil {
		panic(err)
	}
	if err = os.WriteFile(filepath.Join(tmp, "boundary_test.go"), []byte(test), 0o600); err != nil {
		panic(err)
	}
	cmd := exec.Command("go", "test", "-v", "boundary.go", "boundary_test.go")
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "GO111MODULE=off", "GOWORK=off", "GOTOOLCHAIN=local", "GOENV=off", "GOPROXY=off")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("Isolated implementation proof preserved at %s\n", tmp)
	if err = cmd.Run(); err != nil {
		fail("isolated tests: %v", err)
	}
	fmt.Println("LIMIT: package tests, transitive dependency graph, binary symbols, reproducibility and scanner still need the admitted full module cache.")
}
