package typescript

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEmitStruct(t *testing.T) {
	s := Struct{Name: "Animal", Fields: []Field{
		{Name: "id", Type: PrimitiveType{InnerType: "BigInt"}},
		{Name: "fur_colour", Type: PrimitiveType{InnerType: "string", IsNullable: true}},
	}, Comment: "Contains details about animals"}

	expectedAnimalCode := `// Contains details about animals
export interface Animal {
  id: BigInt;
  fur_colour: string | null;
}

`

	assert.Equal(t, expectedAnimalCode, emitString(&s))
}

func TestPrimitiveType_EmitCode(t *testing.T) {
	nullableStringArray := PrimitiveType{
		InnerType:  "string",
		IsArray:    true,
		IsNullable: true,
	}
	assert.Equal(t, "string[] | null", emitString(&nullableStringArray))

	numberArray := PrimitiveType{
		InnerType:  "number",
		IsArray:    true,
		IsNullable: false,
	}
	assert.Equal(t, "number[]", emitString(&numberArray))

	enumType := PrimitiveType{
		InnerType: "DayOfWeek",
	}
	assert.Equal(t, "DayOfWeek", emitString(&enumType))

	void := PrimitiveType{}
	assert.Equal(t, "void", emitString(&void))
}

func emitString(e CodeEmitter) string {
	b := new(bytes.Buffer)
	e.EmitCode(b)
	return b.String()
}
