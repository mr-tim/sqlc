package typescript

import (
	"fmt"
	"github.com/kyleconroy/sqlc/internal/plugin"
	"io"
	"strings"
)

type Comment string

func (c *Comment) EmitCode(w io.Writer) {
	if len(strings.TrimSpace(string(*c))) > 0 {
		for _, line := range strings.Split(string(*c), "\n") {
			fmt.Fprintf(w, "// %s\n", line)
		}
	}
}

type Constant struct {
	Name  string
	Type  string
	Value string
}

type Enum struct {
	Name      string
	Comment   Comment
	Constants []Constant
}

func (e *Enum) EmitCode(w io.Writer) {
	e.Comment.EmitCode(w)
	fmt.Fprintf(w, "export enum %s {\n", e.Name)
	for _, c := range e.Constants {
		fmt.Fprintf(w, "  %s = '%s',\n", c.Name, c.Value)
	}
	fmt.Fprintf(w, "}\n\n")
}

type Struct struct {
	Table   plugin.Identifier
	Name    string
	Fields  []Field
	Comment Comment
}

func (s *Struct) EmitCode(w io.Writer) {
	s.Comment.EmitCode(w)
	fmt.Fprintf(w, "export interface %s {\n", s.Name)
	for _, field := range s.Fields {
		io.WriteString(w, "  ")
		io.WriteString(w, field.Name)
		io.WriteString(w, ": ")
		field.Type.EmitCode(w)
		io.WriteString(w, ";\n")
	}
	fmt.Fprintf(w, "}\n\n")
}

type Field struct {
	Name    string
	Type    PrimitiveType
	Comment string
}

type PrimitiveType struct {
	InnerType  string
	IsArray    bool
	IsNullable bool
}

func (t *PrimitiveType) EmitCode(w io.Writer) {
	if t.InnerType == "" {
		io.WriteString(w, "void")
	} else {
		io.WriteString(w, t.InnerType)
	}
	if t.IsArray {
		io.WriteString(w, "[]")
	}
	if t.IsNullable {
		io.WriteString(w, " | null")
	}
}

type Query struct {
	Cmd        string
	Comments   []Comment
	MethodName string
	SQL        string
	Ret        QueryValue
	Args       []QueryValue
}

func (q *Query) EmitCode(w io.Writer) {
	io.WriteString(w, "\n")
	for _, c := range q.Comments {
		c.EmitCode(w)
	}
	fmt.Fprintf(w, "  // %s\n", q.Cmd)
	fmt.Fprintf(w, "  async %s (\n", q.MethodName)
	// todo: output args
	for idx, arg := range q.Args {
		fmt.Fprintf(w, "    %s: ", arg.Name)
		if arg.Struct != nil {
			io.WriteString(w, arg.Struct.Name)
		} else {
			arg.Type.EmitCode(w)
		}
		if idx != len(q.Args)-1 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, "\n")
	}
	fmt.Fprintf(w, "  ): Promise<")
	q.Ret.EmitCode(w)
	if q.Cmd == ":many" {
		fmt.Fprintf(w, "[]")
	}
	fmt.Fprintf(w, "> {\n")

	io.WriteString(w, "    const result = await this.client.query(\n")
	fmt.Fprintf(w, "      \"%s\",\n", escapeSql(q.SQL))

	if len(q.Args) > 0 {
		io.WriteString(w, "      [\n")
		for _, arg := range q.Args {
			if arg.Struct != nil {
				// destructure the struct
				for _, f := range arg.Struct.Fields {
					fmt.Fprintf(w, "        %s.%s,\n", arg.Name, f.Name)
				}
			} else {
				fmt.Fprintf(w, "        %s,\n", arg.Name)
			}
		}
		io.WriteString(w, "      ]\n")
	}

	io.WriteString(w, "    );\n")

	if q.Cmd == ":execrows" {
		io.WriteString(w, "    return result.rowCount;\n")
	} else {
		io.WriteString(w, "    return result.rows")
		io.WriteString(w, ".map(row => ")
		if q.Ret.Struct == nil {
			fmt.Fprintf(w, "row['%s']", q.Ret.Name)
		} else {
			io.WriteString(w, "({\n")
			for _, col := range q.Ret.Struct.Fields {
				fmt.Fprintf(w, "      %s: row['%s'],\n", col.Name, col.Name)
			}
			io.WriteString(w, "    })")
		}
		io.WriteString(w, ")")
		if q.Cmd != ":many" {
			io.WriteString(w, "[0]")
		}
		io.WriteString(w, ";\n")
	}

	fmt.Fprintf(w, "  }\n")
}

func escapeSql(sql string) string {
	return strings.Replace(
		strings.Replace(
			strings.Replace(sql, "\\", "\\\\", -1),
			"\"", "\\\"", -1),
		"\n", "\\n", -1)
}

func maybeEmitStruct(w io.Writer, queryValue QueryValue) {
	if queryValue.Struct != nil && queryValue.Emit {
		queryValue.Struct.EmitCode(w)
	}
}

type QueryValue struct {
	Emit   bool
	Name   string
	Struct *Struct
	Type   PrimitiveType
}

func (qv *QueryValue) EmitCode(w io.Writer) {
	if qv.Struct != nil {
		io.WriteString(w, qv.Struct.Name)
		if qv.Type.IsArray {
			io.WriteString(w, "[]")
		}
		if qv.Type.IsNullable {
			io.WriteString(w, " | undefined")
		}
	} else {
		qv.Type.EmitCode(w)
	}
}

type CodeEmitter interface {
	EmitCode(w io.Writer)
}
