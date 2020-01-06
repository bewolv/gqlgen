// plugin package interfaces are EXPERIMENTAL.

package plugin

import (
	"github.com/bewolv/gqlgen/codegen"
	"github.com/bewolv/gqlgen/codegen/config"
)

type Plugin interface {
	Name() string
}

type ConfigMutator interface {
	MutateConfig(cfg *config.Config) error
}

type CodeGenerator interface {
	GenerateCode(cfg *codegen.Data) error
}
