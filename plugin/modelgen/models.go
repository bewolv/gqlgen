package modelgen

import (
	"fmt"
	"go/types"
	"sort"

	"github.com/bewolv/gqlgen/codegen/config"
	"github.com/bewolv/gqlgen/codegen/templates"
	"github.com/bewolv/gqlgen/plugin"
	"github.com/vektah/gqlparser/ast"
)

type BuildMutateHook = func(b *ModelBuild) *ModelBuild

func defaultBuildMutateHook(b *ModelBuild) *ModelBuild {
	return b
}

type ModelBuild struct {
	PackageName string
	Interfaces  []*Interface
	Models      []*Object
	Enums       []*Enum
	Scalars     []string
}

type Interface struct {
	Description string
	Name        string
}

type Object struct {
	Description string
	Name        string
	Fields      []*Field
	Implements  []string
}

type Field struct {
	Description string
	Name        string
	Type        types.Type
	Tag         string
}

type Enum struct {
	Description string
	Name        string
	Values      []*EnumValue
}

type EnumValue struct {
	Description string
	Name        string
}

func New() plugin.Plugin {
	return &Plugin{
		MutateHook: defaultBuildMutateHook,
	}
}

type Plugin struct {
	MutateHook BuildMutateHook
}

var _ plugin.ConfigMutator = &Plugin{}

func (m *Plugin) Name() string {
	return "modelgen"
}

func (m *Plugin) MutateConfig(cfg *config.Config) error {
	if err := cfg.Check(); err != nil {
		return err
	}

	schema, _, err := cfg.LoadSchema()
	if err != nil {
		return err
	}

	err = cfg.Autobind(schema)
	if err != nil {
		return err
	}

	cfg.InjectBuiltins(schema)

	binder, err := cfg.NewBinder(schema)
	if err != nil {
		return err
	}

	b := &ModelBuild{
		PackageName: cfg.Model.Package,
	}

	for _, schemaType := range schema.Types {
		if cfg.Models.UserDefined(schemaType.Name) {
			continue
		}

		switch schemaType.Kind {
		case ast.Interface, ast.Union:
			it := &Interface{
				Description: schemaType.Description,
				Name:        schemaType.Name,
			}

			b.Interfaces = append(b.Interfaces, it)
		case ast.Object, ast.InputObject:
			if schemaType == schema.Query || schemaType == schema.Mutation || schemaType == schema.Subscription {
				continue
			}
			it := &Object{
				Description: schemaType.Description,
				Name:        schemaType.Name,
			}

			for _, implementor := range schema.GetImplements(schemaType) {
				it.Implements = append(it.Implements, implementor.Name)
			}

			for _, field := range schemaType.Fields {
				var typ types.Type
				fieldDef := schema.Types[field.Type.Name()]

				if cfg.Models.UserDefined(field.Type.Name()) {
					typ, err = binder.FindTypeFromName(cfg.Models[field.Type.Name()].Model[0])
					if err != nil {
						return err
					}
				} else {
					switch fieldDef.Kind {
					case ast.Scalar:
						// no user defined model, referencing a default scalar
						typ = types.NewNamed(
							types.NewTypeName(0, cfg.Model.Pkg(), "string", nil),
							nil,
							nil,
						)

					case ast.Interface, ast.Union:
						// no user defined model, referencing a generated interface type
						typ = types.NewNamed(
							types.NewTypeName(0, cfg.Model.Pkg(), templates.ToGo(field.Type.Name()), nil),
							types.NewInterfaceType([]*types.Func{}, []types.Type{}),
							nil,
						)

					case ast.Enum:
						// no user defined model, must reference a generated enum
						typ = types.NewNamed(
							types.NewTypeName(0, cfg.Model.Pkg(), templates.ToGo(field.Type.Name()), nil),
							nil,
							nil,
						)

					case ast.Object, ast.InputObject:
						// no user defined model, must reference a generated struct
						typ = types.NewNamed(
							types.NewTypeName(0, cfg.Model.Pkg(), templates.ToGo(field.Type.Name()), nil),
							types.NewStruct(nil, nil),
							nil,
						)

					default:
						panic(fmt.Errorf("unknown ast type %s", fieldDef.Kind))
					}
				}

				name := field.Name
				if nameOveride := cfg.Models[schemaType.Name].Fields[field.Name].FieldName; nameOveride != "" {
					name = nameOveride
				}

				typ = binder.CopyModifiersFromAst(field.Type, typ)

				if isStruct(typ) && (fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject) {
					typ = types.NewPointer(typ)
				}

				var omitEmpty string = ",omitempty"
				// Each field has a generated json tag with omitEmpty
				var tag string = `json:"` + field.Name + omitEmpty + `"`
				//! use tag object separately and populate with dgraph
				if fd := field.Directives.ForName("json"); fd != nil {

					if na := fd.Arguments.ForName("noOmitEmpty"); na != nil {
						if fr, err := na.Value.Value(nil); err == nil {
							if fr.(bool) == true {
								omitEmpty = ""
							}
						}
					}

					if na := fd.Arguments.ForName("name"); na != nil {
						if fr, err := na.Value.Value(nil); err == nil {
							tag = `json:"` + fr.(string) + omitEmpty + `"`
						}
					}
				}

				if fd := field.Directives.ForName("dgraph"); fd != nil {
					if na := fd.Arguments.ForName("tag"); na != nil {
						if fr, err := na.Value.Value(nil); err == nil {
							tag += ` dgraph:"` + fr.(string) + `"`
						}
					}
				}

				// Add validator tags
				if fd := field.Directives.ForName("validator"); fd != nil {
					if na := fd.Arguments.ForName("tags"); na != nil {
						if fr, err := na.Value.Value(nil); err == nil {
							if fr.(string) != "" {
								tag += ` validator:"` + fr.(string) + `"`
							}
						}
					}
				}

				//! -----------------------------------
				it.Fields = append(it.Fields, &Field{
					Name:        name,
					Type:        typ,
					Description: field.Description,
					Tag:         tag,
				})
			}

			b.Models = append(b.Models, it)
		case ast.Enum:
			it := &Enum{
				Name:        schemaType.Name,
				Description: schemaType.Description,
			}

			for _, v := range schemaType.EnumValues {
				it.Values = append(it.Values, &EnumValue{
					Name:        v.Name,
					Description: v.Description,
				})
			}

			b.Enums = append(b.Enums, it)
		case ast.Scalar:
			b.Scalars = append(b.Scalars, schemaType.Name)
		}
	}

	sort.Slice(b.Enums, func(i, j int) bool { return b.Enums[i].Name < b.Enums[j].Name })
	sort.Slice(b.Models, func(i, j int) bool { return b.Models[i].Name < b.Models[j].Name })
	sort.Slice(b.Interfaces, func(i, j int) bool { return b.Interfaces[i].Name < b.Interfaces[j].Name })

	for _, it := range b.Enums {
		cfg.Models.Add(it.Name, cfg.Model.ImportPath()+"."+templates.ToGo(it.Name))
	}
	for _, it := range b.Models {
		cfg.Models.Add(it.Name, cfg.Model.ImportPath()+"."+templates.ToGo(it.Name))
	}
	for _, it := range b.Interfaces {
		cfg.Models.Add(it.Name, cfg.Model.ImportPath()+"."+templates.ToGo(it.Name))
	}
	for _, it := range b.Scalars {
		cfg.Models.Add(it, "github.com/bewolv/gqlgen/graphql.String")
	}

	if len(b.Models) == 0 && len(b.Enums) == 0 {
		return nil
	}

	if m.MutateHook != nil {
		b = m.MutateHook(b)
	}

	return templates.Render(templates.Options{
		PackageName:     cfg.Model.Package,
		Filename:        cfg.Model.Filename,
		Data:            b,
		GeneratedHeader: true,
	})
}

func isStruct(t types.Type) bool {
	_, is := t.Underlying().(*types.Struct)
	return is
}
