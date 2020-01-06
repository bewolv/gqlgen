package resolvergen

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/bewolv/gqlgen/codegen"
	"github.com/bewolv/gqlgen/codegen/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestPlugin(t *testing.T) {
	_ = syscall.Unlink("testdata/out/resolver.go")

	cfg, err := config.LoadConfig("testdata/gqlgen.yml")
	require.NoError(t, err)
	p := Plugin{}

	data, err := codegen.BuildData(cfg)
	if err != nil {
		panic(err)
	}

	require.NoError(t, p.GenerateCode(data))
	assertNoErrors(t, "github.com/bewolv/gqlgen/plugin/resolvergen/testdata/out")
}

func assertNoErrors(t *testing.T, pkg string) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.LoadTypes,
	}, pkg)
	if err != nil {
		panic(err)
	}

	hasErrors := false
	for _, pkg := range pkgs {
		for _, err := range pkg.Errors {
			hasErrors = true
			fmt.Println(err.Pos + ":" + err.Msg)
		}
	}
	if hasErrors {
		t.Fatal("see compilation errors above")
	}
}
