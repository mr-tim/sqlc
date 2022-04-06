package typescript

import (
	"github.com/kyleconroy/sqlc/internal/codegen/sdk"
	"github.com/kyleconroy/sqlc/internal/plugin"
	"log"
)

func postgresType(req *plugin.CodeGenRequest, col *plugin.Column) string {
	columnType := sdk.DataType(col.Type)

	switch columnType {
	case "bigserial", "serial8", "pg_catalog.serial8", "bigint", "int8", "pg_catalog.int8":
		// todo: these might need to be strings?
		return "string"
	case "serial", "serial4", "pg_catalog.serial4", "smallserial", "serial2", "pg_catalog.serial2", "integer", "int", "int4", "pg_catalog.int4", "smallint", "int2", "pg_catalog.int2":
		return "number"
	case "float", "double precision", "float8", "pg_catalog.float8", "real", "float4", "pg_catalog.float4":
		return "number"
	case "numeric", "pg_catalog.numeric", "money":
		return "string"
	case "boolean", "bool", "pg_catalog.bool":
		return "boolean"
	case "json", "jsonb":
		return "unknown"

	// todo: check these
	case "bytea", "blob", "pg_catalog.bytea":
		return "unknown"
	case "date", "pg_catalog.time", "pg_catalog.timetz", "pg_catalog.timestamp", "pg_catalog.timestamptz",
		"timestamptz", "interval", "pg_catalog.interval", "timestamp with time zone":
		return "string"
	case "text", "pg_catalog.varchar", "pg_catalog.bpchar", "string", "citext":
		return "string"
	case "uuid":
		return "string"
	case "inet", "cidr", "macaddr", "macaddr8":
		return "string"
	case "ltree", "lquery", "ltxtquery":
		return "string"
	default:
		for _, schema := range req.Catalog.Schemas {
			if schema.Name == "pg_catalog" {
				continue
			}
			for _, enum := range schema.Enums {
				var enumName string
				if schema.Name == req.Catalog.DefaultSchema {
					enumName = enum.Name
				} else {
					enumName = schema.Name + "." + enum.Name
				}
				if columnType == enumName {
					if schema.Name == req.Catalog.DefaultSchema {
						return modelName(enum.Name, req.Settings)
					}
					return modelName(schema.Name+"_"+enum.Name, req.Settings)
				}
			}
		}
		log.Printf("unknown PostgreSQL type: %s\n", columnType)
		return "any"
	}
}
