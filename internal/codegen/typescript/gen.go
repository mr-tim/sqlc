package typescript

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/kyleconroy/sqlc/internal/codegen/sdk"
	"github.com/kyleconroy/sqlc/internal/plugin"
	"log"
	"sort"
	"strings"
	"text/template"
)

type Constant struct {
	Name  string
	Type  string
	Value string
}

type Enum struct {
	Name      string
	Comment   string
	Constants []Constant
}

type Struct struct {
	Table   plugin.Identifier
	Name    string
	Fields  []Field
	Comment string
}

type Field struct {
	Name    string
	Type    tsType
	Comment string
}

type tsType struct {
	InnerType string
	IsArray   bool
	IsNull    bool
}

type Query struct {
	Cmd        string
	Comments   []string
	MethodName string
	SQL        string
	Ret        QueryValue
	Args       []QueryValue
}

type QueryValue struct {
	Emit   bool
	Name   string
	Struct *Struct
	Type   tsType
}

//go:embed templates
var templateFS embed.FS

func Generate(req *plugin.CodeGenRequest) (*plugin.CodeGenResponse, error) {
	enums := buildEnums(req)
	models := buildModels(req)
	queries := buildQueries(req, models)

	//if err != nil {
	//	return nil, err
	//}

	templates := template.Must(template.New("templates").Funcs(template.FuncMap{
		"outputTsType": outputTsType,
	}).ParseFS(templateFS, "templates/*"))

	buf := new(bytes.Buffer)

	err := templates.ExecuteTemplate(buf, "header.ts.tmpl", nil)
	if err != nil {
		return nil, err
	}

	for _, enum := range enums {
		err := templates.ExecuteTemplate(buf, "enum.ts.tmpl", enum)
		if err != nil {
			return nil, err
		}
	}

	for _, model := range models {
		err := templates.ExecuteTemplate(buf, "struct.ts.tmpl", model)
		if err != nil {
			return nil, err
		}
	}

	for _, query := range queries {
		err := templates.ExecuteTemplate(buf, "query.ts.tmpl", query)
		if err != nil {
			return nil, err
		}
	}

	files := make([]*plugin.File, 0)
	files = append(files, &plugin.File{Name: "sqlc.ts", Contents: buf.Bytes()})

	result := plugin.CodeGenResponse{Files: files}

	return &result, nil
}

func outputTsType(t tsType) string {
	if t.InnerType == "" {
		return "void"
	} else if t.IsArray {
		return t.InnerType + "[]"
	}
	return t.InnerType
}

func buildEnums(req *plugin.CodeGenRequest) []Enum {
	var enums []Enum
	for _, schema := range req.Catalog.Schemas {
		if schema.Name == "pg_catalog" {
			continue
		}
		for _, enum := range schema.Enums {
			var enumName string
			if schema.Name == req.Catalog.DefaultSchema {
				enumName = enum.Name
			} else {
				enumName = schema.Name + "_" + enum.Name
			}

			e := Enum{
				Name:    modelName(enumName, req.Settings),
				Comment: enum.Comment,
			}
			for _, v := range enum.Vals {
				e.Constants = append(e.Constants, Constant{
					Name:  enumValueName(v),
					Value: v,
					Type:  e.Name,
				})
			}

			enums = append(enums, e)
		}
	}

	if len(enums) > 0 {
		sort.Slice(enums, func(i, j int) bool { return enums[i].Name < enums[j].Name })
	}

	return enums
}

func buildModels(req *plugin.CodeGenRequest) []Struct {
	var structs []Struct
	for _, schema := range req.Catalog.Schemas {
		if schema.Name == "pg_catalog" {
			continue
		}
		for _, table := range schema.Tables {
			var tableName string
			if schema.Name == req.Catalog.DefaultSchema {
				tableName = table.Rel.Name
			} else {
				tableName = schema.Name + "_" + table.Rel.Name
			}
			s := Struct{Table: plugin.Identifier{Schema: schema.Name, Name: table.Rel.Name},
				Name: modelName(tableName, req.Settings), Comment: table.Comment}
			for _, column := range table.Columns {
				typ := makeTsType(req, column)
				typ.InnerType = strings.TrimPrefix(typ.InnerType, "models.")
				s.Fields = append(s.Fields, Field{
					// todo: make
					Name:    column.Name,
					Type:    typ,
					Comment: column.Comment,
				})
			}
			structs = append(structs, s)
		}
	}
	if len(structs) > 0 {
		sort.Slice(structs, func(i, j int) bool { return structs[i].Name < structs[j].Name })
	}
	return structs
}

func makeTsType(req *plugin.CodeGenRequest, col *plugin.Column) tsType {
	typ := tsInnerType(req, col)
	return tsType{
		InnerType: typ,
		IsArray:   col.IsArray,
		IsNull:    !col.NotNull,
	}
}

func tsInnerType(req *plugin.CodeGenRequest, col *plugin.Column) string {
	//columnType := sdk.DataType(col.Type)
	//for _, oride := range req.Settings.Overrides {
	//	if !pyTypeIsSet(oride.PythonType) {
	//		continue
	//	}
	//	sameTable := sdk.Matches(oride, col.Table, req.Catalog.DefaultSchema)
	//	if oride.Column != "" && sdk.MatchString(oride.ColumnName, col.Name) && sameTable {
	//		return pyTypeString(oride.PythonType)
	//	}
	//	if oride.DbType != "" && oride.DbType == columnType && oride.Nullable != (col.NotNull || col.IsArray) {
	//		return pyTypeString(oride.PythonType)
	//	}
	//}

	switch req.Settings.Engine {
	case "postgresql":
		return postgresType(req, col)
	default:
		log.Println("unsupported engine type")
		return "unknown"
	}
}

func buildQueries(req *plugin.CodeGenRequest, structs []Struct) []Query {
	qs := make([]Query, 0)
	for _, query := range req.Queries {
		q := Query{
			Cmd:        query.Cmd,
			Comments:   query.Comments,
			MethodName: sdk.LowerTitle(query.Name),
			SQL:        query.Text,
		}

		if len(query.Params) > 4 {
			var cols []tsColumn
			for _, p := range query.Params {
				cols = append(cols, tsColumn{id: p.Number, Column: p.Column})
			}
			q.Args = []QueryValue{{
				Emit:   true,
				Name:   "arg",
				Struct: columnsToStruct(req, query.Name+"Params", cols),
			}}
		} else {
			args := make([]QueryValue, 0, len(query.Params))
			for _, p := range query.Params {
				args = append(args, QueryValue{
					Name: paramName(p),
					Type: makeTsType(req, p.Column),
				})
			}
			q.Args = args
		}

		if len(query.Columns) == 1 {
			c := query.Columns[0]
			q.Ret = QueryValue{
				Name: columnName(c, 0),
				Type: makeTsType(req, c),
			}
		} else if len(query.Columns) > 1 {
			var gs *Struct
			var emit bool

			for _, s := range structs {
				if len(s.Fields) != len(query.Columns) {
					continue
				}
				same := true

				for i, f := range s.Fields {
					c := query.Columns[i]
					// HACK: models do not have "models." on their types, so trim that so we can find matches
					trimmedPyType := makeTsType(req, c)
					trimmedPyType.InnerType = strings.TrimPrefix(trimmedPyType.InnerType, "models.")
					sameName := f.Name == columnName(c, i)
					sameType := f.Type == trimmedPyType
					sameTable := sdk.SameTableName(c.Table, &s.Table, req.Catalog.DefaultSchema)
					if !sameName || !sameType || !sameTable {
						same = false
					}
				}
				if same {
					gs = &s
					break
				}
			}

			if gs == nil {
				var columns []tsColumn
				for i, c := range query.Columns {
					columns = append(columns, tsColumn{
						id:     int32(i),
						Column: c,
					})
				}
				gs = columnsToStruct(req, query.Name+"Row", columns)
				emit = true
			}
			q.Ret = QueryValue{
				Emit:   emit,
				Name:   "i",
				Struct: gs,
			}
		}

		qs = append(qs, q)
	}
	return qs
}

type tsColumn struct {
	id int32
	*plugin.Column
}

func columnsToStruct(req *plugin.CodeGenRequest, name string, columns []tsColumn) *Struct {
	gs := Struct{
		Name: name,
	}
	seen := map[string]int32{}
	suffixes := map[int32]int32{}
	for i, c := range columns {
		colName := columnName(c.Column, i)
		fieldName := colName
		// Track suffixes by the ID of the column, so that columns referring to
		// the same numbered parameter can be reused.
		var suffix int32
		if o, ok := suffixes[c.id]; ok {
			suffix = o
		} else if v := seen[colName]; v > 0 {
			suffix = v + 1
		}
		suffixes[c.id] = suffix
		if suffix > 0 {
			fieldName = fmt.Sprintf("%s_%d", fieldName, suffix)
		}
		gs.Fields = append(gs.Fields, Field{
			Name: fieldName,
			Type: makeTsType(req, c.Column),
		})
		seen[colName]++
	}
	return &gs
}

func columnName(c *plugin.Column, pos int) string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("column_%d", pos+1)
}

func paramName(p *plugin.Parameter) string {
	if p.Column.Name != "" {
		return p.Column.Name
	}
	return fmt.Sprintf("dollar_%d", p.Number)
}

func modelName(name string, settings *plugin.Settings) string {
	if rename := settings.Rename[name]; rename != "" {
		return rename
	}
	return underScoresToCamelCase(name)
}

func enumValueName(name string) string {
	return underScoresToCamelCase(name)
}

func underScoresToCamelCase(name string) string {
	out := ""
	for _, p := range strings.Split(name, "_") {
		out += strings.Title(p)
	}
	return out
}
