package gen_entity

import (
	"google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
	"text/template"
)

func parseAndExec(g *protogen.GeneratedFile, temp string, ctx TemplateCtx) {
	tp, err := template.New("parse-field-idx").Parse(temp)
	if err != nil {
		panic(err)
	}
	err = tp.Execute(g, ctx)
	if err != nil {
		panic(err)
	}
}

type TemplateCtx struct {
	FieldName  string
	FieldCount int
}

const templateParentField = `
	_parent Entity       // current struct 's parent
	_idx_in_parent int    // current 's index in parent struct
	_dt int   // struct in parent type:1 direct  2  slice  3 map value
	`

func GenerateParentFields(g *protogen.GeneratedFile, f *internal_gengo.FileInfo, m *internal_gengo.MessageInfo) {
	_, err := g.Write([]byte(templateParentField))
	if err != nil {
		panic(err)
	}
}

const templateIdxField = `_idx_{{.FieldName}} int`

func GenerateMsgFieldIdx(g *protogen.GeneratedFile, f *internal_gengo.FileInfo, m *internal_gengo.MessageInfo, field *protogen.Field) {
	parseAndExec(g, templateIdxField, TemplateCtx{FieldName: field.GoName})
}

const templateStructDirtyField = `_dirty_flag [{{.FieldCount}}]byte`

func GenerateMsgFieldDirty(g *protogen.GeneratedFile, fc int) {
	parseAndExec(g, templateStructDirtyField, TemplateCtx{FieldCount: fc})
}
