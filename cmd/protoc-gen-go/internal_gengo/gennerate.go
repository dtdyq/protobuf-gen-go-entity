package internal_gengo

import (
	"fmt"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"strconv"
	"strings"
	"text/template"
)

type StructFieldCategory int

const (
	Basic StructFieldCategory = iota
	BasicSlice
	BasicMap
	StructField
	StructSlice
	StructMap
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
	StructName string
	FieldName  string
	FieldCount int
}

func GenerateMsgMapFieldsDeleted(f *fileInfo, m *messageInfo) {

}

const templateParentField = `
	_parent Entity       // current struct 's parent
	_idx_in_parent int    // current 's index in parent struct
	_dt byte   // struct in parent type:1 direct  2  slice  3 map value
`

func GenerateParentFields(g *protogen.GeneratedFile, fc int) {
	_, err := g.Write([]byte(templateParentField))
	if err != nil {
		panic(err)
	}
}

const templateIdxField = `
_idx_{{.FieldName}} int
`

func GenerateMsgFieldIdx(g *protogen.GeneratedFile, field *protogen.Field) {
	parseAndExec(g, templateIdxField, TemplateCtx{FieldName: field.GoName})
}

const templateStructDirtyField = `
_dirty_flag [{{.FieldCount}}]byte
`

func GenerateMsgFieldDirty(g *protogen.GeneratedFile, fc int) {
	parseAndExec(g, templateStructDirtyField, TemplateCtx{FieldCount: fc})
}

const templateStructProtoReflectField = `
_prm protoreflect.Message
`

func GenerateMsgFieldProtoReflect(g *protogen.GeneratedFile, fc int) {
	parseAndExec(g, templateStructProtoReflectField, TemplateCtx{FieldCount: fc})
}

const templateStructParentMethodMarkDirty = `
func (x *{{.StructName}}) MarkDirty(idx int ,dt byte) {
	if x != nil {
		x._dirty_flag[idx] = dt
		if x._parent != nil && dt >0 {
			x._parent.MarkDirty(x._idx_in_parent,x._dt)
		}
	}
}
`

const templateStructParentMethodSetParent = `
func (x *{{.StructName}}) SetParent(entity Entity,idx int,dt byte) {
	if x != nil {
		x._parent = entity
		x._idx_in_parent = idx
		x._dt = dt
	}
}
`

func GenerateMsgParentMethod(g *protogen.GeneratedFile, f *fileInfo, m *messageInfo) {
	preLine(g)
	parseAndExec(g, templateStructParentMethodMarkDirty, TemplateCtx{StructName: m.GoIdent.GoName})
	parseAndExec(g, templateStructParentMethodSetParent, TemplateCtx{StructName: m.GoIdent.GoName})

	// generate BuildEntityInfo
	g.P(fmt.Sprintf("func (x *%s)  BuildEntityInfo(){", m.GoIdent.GoName))
	g.P("if x!= nil {")

	for idx, field := range getFieldsNoDeletedKey(m) {
		g.P(fmt.Sprintf("x._idx_%s = %d", field.GoName, idx))
		if isStructMap(field) {
			g.P(fmt.Sprintf("for _,v := range x.%s {", field.GoName))
			g.P(fmt.Sprintf("v.SetParent(x,x._idx_%s,3)", field.GoName))
			g.P(fmt.Sprintf("v.BuildEntityInfo()"))
			g.P("}")
		} else if isStructSlice(field) {
			g.P(fmt.Sprintf("for _,v := range x.%s {", field.GoName))
			g.P(fmt.Sprintf("v.SetParent(x,x._idx_%s,3)", field.GoName))
			g.P(fmt.Sprintf("v.BuildEntityInfo()"))
			g.P("}")
		} else if isJustStruct(field) {
			g.P(fmt.Sprintf("if x.%s!= nil {", field.GoName))
			g.P(fmt.Sprintf("x.%s.SetParent(x,x._idx_%s,1)", field.GoName, field.GoName))
			g.P(fmt.Sprintf("x.%s.BuildEntityInfo()", field.GoName))
			g.P("}")

		}
	}
	g.P("}")
	g.P("}")

	// generate BuildEntityInfo
	g.P(fmt.Sprintf("func (x *%s)  MarkAllDirty(dt byte){", m.GoIdent.GoName))
	g.P("if x!= nil {")
	for _, field := range getFieldsNoDeletedKey(m) {
		if isStructMap(field) {
			g.P(fmt.Sprintf("for _,v := range x.%s {", field.GoName))
			g.P(fmt.Sprintf("v.MarkAllDirty(dt)"))
			g.P("}")
		} else if isStructSlice(field) {
			g.P(fmt.Sprintf("for _,v := range x.%s {", field.GoName))
			g.P(fmt.Sprintf("v.MarkAllDirty(dt)"))
			g.P("}")
		} else if isJustStruct(field) {
			g.P(fmt.Sprintf("if x.%s!= nil {", field.GoName))
			g.P(fmt.Sprintf("x.%s.MarkAllDirty(dt)", field.GoName))
			g.P("}")
		}
	}
	g.P(fmt.Sprintf("for i,_ := range x._dirty_flag {"))
	g.P(fmt.Sprintf("x._dirty_flag[i] = dt"))
	g.P("}")

	g.P("if dt == 0 {")
	for _, field := range getFieldsOnlyDeletedKey(m) {
		goType, _ := fieldGoType(g, f, field)
		g.P(fmt.Sprintf("x.%s = make(%s,0)", field.GoName, goType))
	}
	g.P("}")

	g.P("}")
	g.P("}")

	GenerateMsgDirtyOperationMethod(g, f, m)
}

const templateStructGetProtoReflectMethod = `
func (x *{{.StructName}}) GetProtoReflect() protoreflect.Message {
	if x._prm == nil {
		x._prm = x.ProtoReflect()
	}
	return x._prm
}
`

func GenerateMsgDirtyOperationMethod(g *protogen.GeneratedFile, f *fileInfo, m *messageInfo) {
	// collect dirty field
	preLine(g)
	// ===========================================================

	parseAndExec(g, templateStructGetProtoReflectMethod, TemplateCtx{StructName: m.GoIdent.GoName})
	// ===========================================================

	g.P(fmt.Sprintf("func (x *%s) hasField(fn protoreflect.Name) bool {", m.GoIdent.GoName))
	g.P("return x.GetProtoReflect().Has(x.GetProtoReflect().Descriptor().Fields().ByName(fn))")
	g.P("}")
	// ===========================================================

	g.P(fmt.Sprintf("func (x *%s) MergeFrom(s *%s) {", m.GoIdent.GoName, m.GoIdent.GoName))
	g.P(fmt.Sprintf("if x == nil || s == nil {"))
	g.P("return")
	g.P("}")

	for _, field := range getFieldsNoDeletedKey(m) {
		//goType, _ := fieldGoType(g, f, field)

		switch getFieldCategory(field) {
		case Basic:
			g.P(fmt.Sprintf("if s.hasField(`%s`) {", field.GoName))
			g.P(fmt.Sprintf("x.Set%s(s.Get%s())", field.GoName, field.GoName))
			g.P("}")
		case StructField:
			g.P(fmt.Sprintf("if s.hasField(`%s`) {", field.GoName))
			g.P(fmt.Sprintf("x.Get%s().MergeFrom(s.Get%s())", field.GoName, field.GoName))
			g.P("}")
		}
	}

	g.P("}")

	// ===========================================================

	g.P(fmt.Sprintf("func (x *%s) IsDirty() bool {", m.GoIdent.GoName))
	g.P("for _,b := range x._dirty_flag {")
	g.P("if b > 0 {")
	g.P("return true")
	g.P("}")
	g.P("}")
	g.P("return false")
	g.P("}")
	// ===========================================================
	g.P(fmt.Sprintf("func (x *%s) CopyAll(r *%s) {", m.GoIdent.GoName, m.GoIdent.GoName))
	g.P("if x != nil && r != nil {")
	g.P("bys,_ := proto.Marshal(x)")
	g.P("proto.Unmarshal(bys,r)")
	g.P("}")
	g.P("}")
	// ===========================================================
	g.P(fmt.Sprintf("func (x *%s) CollectDirty(r *%s,clear bool) {", m.GoIdent.GoName, m.GoIdent.GoName))
	g.P("if x != nil && r != nil {")
	g.P("r.BuildEntityInfo()")
	var fc = 0
	for _, field := range getFieldsNoDeletedKey(m) {
		fc += 1
		goType, _ := fieldGoType(g, f, field)
		switch getFieldCategory(field) {
		case Basic: //数组和基础类型，直接替换，一定是全量更新，数组内每个字段修改都会致整个数组脏
			g.P(fmt.Sprintf("if x._dirty_flag[x._idx_%s] > 0 {", field.GoName))
			g.P(fmt.Sprintf("r.Set%s(x.%s)", field.GoName, field.GoName))
			g.P("}")
		case BasicSlice:
			g.P(fmt.Sprintf("if x._dirty_flag[x._idx_%s] > 0 {", field.GoName))
			g.P(fmt.Sprintf("r.%s = make(%s,0)", field.GoName, goType))
			g.P(fmt.Sprintf("if x.%s != nil {", field.GoName))
			g.P(fmt.Sprintf("r.Set%s(x.%s)", field.GoName, field.GoName))
			g.P("}")
			g.P("}")
		case StructSlice:
			g.P(fmt.Sprintf("if x._dirty_flag[x._idx_%s] > 0 {", field.GoName))
			g.P(fmt.Sprintf("r.%s = make(%s,0)", field.GoName, goType))
			g.P(fmt.Sprintf("if x.%s != nil{", field.GoName))
			g.P(fmt.Sprintf("for _,i := range x.%s {", field.GoName))
			g.P(fmt.Sprintf("if i != nil {"))
			g.P(fmt.Sprintf("vi := &%s{}", strings.TrimPrefix(goType, "[]*")))
			g.P(fmt.Sprintf("i.CopyAll(vi)"))
			g.P(fmt.Sprintf("r.Append%s(vi)", field.GoName))
			g.P("}")
			g.P("}")
			g.P("}")
			g.P("}")
		case StructField: // 收集结构体脏字
			g.P(fmt.Sprintf("if x._dirty_flag[x._idx_%s] > 0 {", field.GoName))
			g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
			g.P(fmt.Sprintf("r.%s = nil", field.GoName))
			g.P(fmt.Sprintf("} else {"))
			g.P(fmt.Sprintf("if r.%s == nil {", field.GoName))
			g.P(fmt.Sprintf("r.%s = &%s{}", field.GoName, strings.TrimPrefix(goType, "*")))
			g.P("}")
			g.P(fmt.Sprintf("x.%s.CollectDirty(r.%s,clear)", field.GoName, field.GoName))
			g.P("}")

			g.P("}")
		case StructMap: //todo merge deleted key
			valType, _ := fieldGoType(g, f, field.Message.Fields[1])
			fieldDeleted := getFieldDeletedOfField(m, field)
			fieldDeletedGoType, _ := fieldGoType(g, f, fieldDeleted)
			g.P(fmt.Sprintf("if x._dirty_flag[x._idx_%s] > 0 {", field.GoName))
			//g.P(fmt.Sprintf("r.%s = make(%s)", field.GoName, goType))
			//g.P(fmt.Sprintf("r.%s = x.%s", fieldDeleted.GoName, fieldDeleted.GoName))
			g.P(fmt.Sprintf("r.MarkDirty(r._idx_%s,1)", field.GoName))
			g.P(fmt.Sprintf("if x.%s != nil {", field.GoName))
			g.P(fmt.Sprintf("if r.%s == nil {", fieldDeleted.GoName))
			g.P(fmt.Sprintf("r.%s = make(%s,0)", fieldDeleted.GoName, fieldDeletedGoType))
			g.P("}")
			g.P(fmt.Sprintf("if r.%s == nil {", field.GoName))
			g.P(fmt.Sprintf("r.%s = make(%s,0)", field.GoName, goType))
			g.P("}")
			g.P(fmt.Sprintf("r.%s = MergeNotExistInto(r.%s,x.%s)", fieldDeleted.GoName, fieldDeleted.GoName, fieldDeleted.GoName))

			g.P(fmt.Sprintf("for k,v := range x.%s {", field.GoName))
			g.P(fmt.Sprintf("if v != nil && v.IsDirty() {"))
			g.P(fmt.Sprintf("var vr,ok = r.%s[k]", field.GoName))
			g.P(fmt.Sprintf("if !ok {"))
			g.P(fmt.Sprintf("vr = &%s{}", strings.TrimPrefix(valType, "*")))
			g.P(fmt.Sprintf("r.%s = RemoveValueOf(r.%s,k)", fieldDeleted.GoName, fieldDeleted.GoName))
			g.P("}")
			g.P(fmt.Sprintf("v.CollectDirty(vr,clear)"))
			g.P(fmt.Sprintf("r.%s[k] =vr", field.GoName))
			g.P("}")
			g.P("}")

			g.P(fmt.Sprintf("for _,d := range r.%s {", fieldDeleted.GoName))
			g.P(fmt.Sprintf("delete(r.%s,d)", field.GoName))
			g.P("}")

			g.P("} else {")
			g.P(fmt.Sprintf("r.%s = nil", field.GoName))
			g.P("}")
			g.P("}")
		case BasicMap:
			fieldDeleted := getFieldDeletedOfField(m, field)
			fieldDeletedGoType, _ := fieldGoType(g, f, fieldDeleted)
			g.P(fmt.Sprintf("if x._dirty_flag[x._idx_%s] > 0 {", field.GoName))
			//g.P(fmt.Sprintf("r.%s = make(%s)", field.GoName, goType))
			//g.P(fmt.Sprintf("r.%s = x.%s", fieldDeleted.GoName, fieldDeleted.GoName))
			g.P(fmt.Sprintf("r.MarkDirty(r._idx_%s,1)", field.GoName))

			g.P(fmt.Sprintf("if x.%s != nil {", field.GoName))

			g.P(fmt.Sprintf("if r.%s == nil {", fieldDeleted.GoName))
			g.P(fmt.Sprintf("r.%s = make(%s,0)", fieldDeleted.GoName, fieldDeletedGoType))
			g.P("}")

			g.P(fmt.Sprintf("if r.%s == nil {", field.GoName))
			g.P(fmt.Sprintf("r.%s = make(%s,0)", field.GoName, goType))
			g.P("}")

			g.P(fmt.Sprintf("r.%s = MergeNotExistInto(r.%s,x.%s)", fieldDeleted.GoName, fieldDeleted.GoName, fieldDeleted.GoName))

			g.P(fmt.Sprintf("for k,v := range x.%s {", field.GoName))
			g.P(fmt.Sprintf("var _,ok = r.%s[k]", field.GoName))
			g.P(fmt.Sprintf("if !ok {"))
			g.P(fmt.Sprintf("r.%s = RemoveValueOf(r.%s,k)", fieldDeleted.GoName, fieldDeleted.GoName))
			g.P("}")
			g.P(fmt.Sprintf("r.Put%sByKey(k, v)", field.GoName))
			g.P("}")

			g.P(fmt.Sprintf("for _,d := range r.%s {", fieldDeleted.GoName))
			g.P(fmt.Sprintf("delete(r.%s,d)", field.GoName))
			g.P("}")

			g.P("} else {")
			g.P(fmt.Sprintf("r.%s = nil", field.GoName))
			g.P("}")
			g.P("}")
		}
	}
	g.P("if clear {")
	g.P(fmt.Sprintf("x._dirty_flag = [%d]byte{}", fc))
	g.P("}")
	g.P("}")
	g.P("}")

	// ===========================================================
}

func getFieldDeletedOfField(m *messageInfo, field *protogen.Field) *protogen.Field {
	for _, f := range m.Fields {
		if strings.HasSuffix(f.GoName, "Deleted") && strings.HasPrefix(f.GoName, field.GoName) {
			return f
		}
	}
	panic("no deleted field for " + field.GoName)
}

func preLine(g *protogen.GeneratedFile) {
	g.P("// entity block start")
	g.P("")
}

// GenerateMsgFieldMethod 生成结构体内字段的set等方法
func GenerateMsgFieldMethod(g *protogen.GeneratedFile, f *fileInfo, m *messageInfo) {
	preLine(g)
	for _, field := range getFieldsNoDeletedKey(m) {
		switch getFieldCategory(field) {
		case Basic:
			GenerateMsgBasicFieldOperationMethod(g, f, m, field)
		case StructField:
			GenerateMsgStructFieldOperationMethod(g, f, m, field)
		case BasicSlice:
			GenerateMsgBasicSliceFieldOperationMethod(g, f, m, field)
		case StructSlice:
			GenerateMsgStructSliceFieldOperationMethod(g, f, m, field)
		case BasicMap:
			GenerateMsgBasicMapFieldOperationMethod(g, f, m, field)
		case StructMap:
			GenerateMsgStructMapFieldOperationMethod(g, f, m, field)
		}
	}
}

func GenerateMsgStructMapFieldOperationMethod(g *protogen.GeneratedFile, f *fileInfo, m *messageInfo, field *protogen.Field) {
	keyType, _ := fieldGoType(g, f, field.Message.Fields[0])
	valType, _ := fieldGoType(g, f, field.Message.Fields[1])
	fieldDeleted := getFieldDeletedOfField(m, field)
	defVal := "nil"
	// =========================================
	goType, _ := fieldGoType(g, f, field)
	g.P(fmt.Sprintf("func (x *%s) Get%s() %s{", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("return x.%s", field.GoName))
	g.P("}")
	g.P("return nil")
	g.P("}")
	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Get%sByKey(k %s) (%s,bool) {", m.GoIdent.GoName, field.GoName, keyType, valType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("ret,ok := x.%s[k]", field.GoName))
	g.P(fmt.Sprintf("return ret,ok"))
	g.P("}")
	g.P(fmt.Sprintf("return %s,false", defVal))
	g.P("}")
	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Set%s(v %s) {", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")

	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	//v 是不是空都需要置脏位，去掉所有旧的
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P(fmt.Sprintf("for o,_ := range x.%s {", field.GoName))
	g.P(fmt.Sprintf("x.%s = append(x.%s,o)", fieldDeleted.GoName, fieldDeleted.GoName))
	g.P(fmt.Sprintf("}"))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))

	g.P(fmt.Sprintf("if v != nil {"))
	g.P(fmt.Sprintf("for k,_ := range v {"))
	g.P(fmt.Sprintf("x.Put%sByKey(k,v[k])", field.GoName))
	g.P("}")
	g.P("}")

	g.P("}")
	g.P("}")
	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Put%sByKey(k %s,v %s) {", m.GoIdent.GoName, field.GoName, keyType, valType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("x.%s[k] = v", field.GoName))
	g.P(fmt.Sprintf("x.%s = RemoveValueOf(x.%s,k)", fieldDeleted.GoName, fieldDeleted.GoName))
	g.P(fmt.Sprintf("if x.%s[k] != nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s[k].SetParent(x,x._idx_%s,1)", field.GoName, field.GoName))
	g.P(fmt.Sprintf("x.%s[k].BuildEntityInfo()", field.GoName))
	g.P(fmt.Sprintf("x.%s[k].MarkAllDirty(1)", field.GoName))
	g.P("}")

	g.P("}")
	g.P("}")

	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Del%sByKey(k %s) {", m.GoIdent.GoName, field.GoName, keyType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("_,ok := x.%s[k]", field.GoName))
	g.P(fmt.Sprintf("if ok {"))
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P(fmt.Sprintf("delete(x.%s,k)", field.GoName))
	g.P(fmt.Sprintf("x.%s = AppendIfNotExist(x.%s,k)", fieldDeleted.GoName, fieldDeleted.GoName))
	g.P()
	g.P("}")
	g.P("}")
	g.P("}")

}
func GenerateMsgBasicMapFieldOperationMethod(g *protogen.GeneratedFile, f *fileInfo, m *messageInfo, field *protogen.Field) {
	keyType, _ := fieldGoType(g, f, field.Message.Fields[0])
	valType, _ := fieldGoType(g, f, field.Message.Fields[1])
	defVal := fieldDefaultValue(g, f, m, field.Message.Fields[1])
	fieldDeleted := getFieldDeletedOfField(m, field)
	// =========================================
	goType, _ := fieldGoType(g, f, field)
	g.P(fmt.Sprintf("func (x *%s) Get%s() %s{", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("return x.%s", field.GoName))
	g.P("}")
	g.P("return nil")
	g.P("}")
	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Get%sByKey(k %s) (%s,bool) {", m.GoIdent.GoName, field.GoName, keyType, valType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("ret,ok := x.%s[k]", field.GoName))
	g.P(fmt.Sprintf("return ret,ok"))
	g.P("}")
	g.P(fmt.Sprintf("return %s,false", defVal))
	g.P("}")
	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Set%s(v %s) {", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")

	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	//v 是不是空都需要置脏位，去掉所有旧的
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P(fmt.Sprintf("for o,_ := range x.%s {", field.GoName))
	g.P(fmt.Sprintf("x.%s = append(x.%s,o)", fieldDeleted.GoName, fieldDeleted.GoName))
	g.P(fmt.Sprintf("}"))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))

	g.P(fmt.Sprintf("if v != nil {"))
	g.P(fmt.Sprintf("for k,_ := range v {"))
	g.P(fmt.Sprintf("x.Put%sByKey(k,v[k])", field.GoName))
	g.P("}")
	g.P("}")

	g.P("}")
	g.P("}")
	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Put%sByKey(k %s,v %s) {", m.GoIdent.GoName, field.GoName, keyType, valType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("x.%s[k] = v", field.GoName))
	g.P(fmt.Sprintf("x.%s = RemoveValueOf(x.%s,k)", fieldDeleted.GoName, fieldDeleted.GoName))
	g.P("}")
	g.P("}")

	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Del%sByKey(k %s) {", m.GoIdent.GoName, field.GoName, keyType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("_,ok := x.%s[k]", field.GoName))
	g.P(fmt.Sprintf("if ok {"))
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P(fmt.Sprintf("delete(x.%s,k)", field.GoName))
	g.P(fmt.Sprintf("x.%s = AppendIfNotExist(x.%s,k)", fieldDeleted.GoName, fieldDeleted.GoName))
	g.P()
	g.P("}")
	g.P("}")
	g.P("}")

}

func GenerateMsgStructSliceFieldOperationMethod(g *protogen.GeneratedFile, f *fileInfo, m *messageInfo, field *protogen.Field) {
	goType, _ := fieldGoType(g, f, field)
	oriType := strings.TrimPrefix(goType, "[]")
	//defaultValue := getDefaultValByKind(g, f, field, field.Desc.Kind())
	g.P(fmt.Sprintf("func (x *%s) Get%s() %s{", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s,0)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("return x.%s", field.GoName))
	g.P("}")
	g.P("return nil")
	g.P("}")
	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Set%s(v %s) {", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("x.%s = v", field.GoName))
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))

	g.P(fmt.Sprintf("if v == nil {"))
	g.P(fmt.Sprintf("x.%s = make(%s,0)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("for i,_ := range x.%s {", field.GoName))
	g.P(fmt.Sprintf("if x.%s[i] != nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s[i].SetParent(x,x._idx_%s,1)", field.GoName, field.GoName))
	g.P(fmt.Sprintf("x.%s[i].BuildEntityInfo()", field.GoName))
	g.P(fmt.Sprintf("x.%s[i].MarkAllDirty(1)", field.GoName))
	g.P("}")
	g.P("}")

	g.P("}")
	g.P("}")
	// =========================================
	g.P(fmt.Sprintf("func (x *%s) Get%sByIdx(i int) %s{", m.GoIdent.GoName, field.GoName, oriType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s,0)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("if len(x.%s) > i {", field.GoName))
	g.P(fmt.Sprintf("return x.%s[i]", field.GoName))
	g.P("}")
	g.P(fmt.Sprintf("return &%s{}", strings.TrimPrefix(oriType, "*")))
	g.P("}")
	g.P("return nil")
	g.P("}")
	// =========================================

	g.P(fmt.Sprintf("func (x *%s) Append%s(v %s) {", m.GoIdent.GoName, field.GoName, oriType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s,0)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("x.%s = append(x.%s,v)", field.GoName, field.GoName))
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P(fmt.Sprintf("v.SetParent(x,x._idx_%s,1)", field.GoName))
	g.P(fmt.Sprintf("v.BuildEntityInfo()"))
	g.P(fmt.Sprintf("v.MarkAllDirty(1)"))
	g.P("}")
	g.P("}")

}

func GenerateMsgBasicSliceFieldOperationMethod(g *protogen.GeneratedFile, f *fileInfo, m *messageInfo, field *protogen.Field) {
	goType, _ := fieldGoType(g, f, field)
	oriType := strings.TrimPrefix(goType, "[]")
	defaultValue := getDefaultValByKind(g, f, field, field.Desc.Kind())
	g.P(fmt.Sprintf("func (x *%s) Get%s() %s{", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s,0)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("return x.%s", field.GoName))
	g.P("}")
	g.P("return nil")
	g.P("}")

	g.P(fmt.Sprintf("func (x *%s) Set%s(v %s) {", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("x.%s = v", field.GoName))
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P("}")
	g.P("}")

	g.P(fmt.Sprintf("func (x *%s) Append%s(v %s) {", m.GoIdent.GoName, field.GoName, oriType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s,0)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("x.%s = append(x.%s,v)", field.GoName, field.GoName))
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P("}")
	g.P("}")

	g.P(fmt.Sprintf("func (x *%s) Get%sByIdx(i int) %s{", m.GoIdent.GoName, field.GoName, oriType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = make(%s,0)", field.GoName, goType))
	g.P("}")
	g.P(fmt.Sprintf("if len(x.%s) > i {", field.GoName))
	g.P(fmt.Sprintf("return x.%s[i]", field.GoName))
	g.P("}")
	g.P("}")
	g.P(fmt.Sprintf("return %s", defaultValue))
	g.P("}")
}

func GenerateMsgStructFieldOperationMethod(g *protogen.GeneratedFile, f *fileInfo, m *messageInfo, field *protogen.Field) {
	goType, _ := fieldGoType(g, f, field)

	g.P(fmt.Sprintf("func (x *%s) Get%s() %s{", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("if x.%s == nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s = &%s{}", field.GoName, strings.TrimPrefix(goType, "*")))
	g.P("}")
	g.P(fmt.Sprintf("x.%s.SetParent(x,x._idx_%s,1)", field.GoName, field.GoName))
	g.P(fmt.Sprintf("x.%s.BuildEntityInfo()", field.GoName))
	g.P(fmt.Sprintf("return x.%s", field.GoName))
	g.P("}")
	g.P("return nil")
	g.P("}")

	g.P(fmt.Sprintf("func (x *%s) Set%s(v %s) {", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("x.%s = v", field.GoName))
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P(fmt.Sprintf("if x.%s != nil {", field.GoName))
	g.P(fmt.Sprintf("x.%s.SetParent(x,x._idx_%s,1)", field.GoName, field.GoName))
	g.P(fmt.Sprintf("x.%s.BuildEntityInfo()", field.GoName))
	g.P(fmt.Sprintf("x.%s.MarkAllDirty(1)", field.GoName))
	g.P("}")
	g.P("}")
	g.P("}")
}

func GenerateMsgBasicFieldOperationMethod(g *protogen.GeneratedFile, f *fileInfo, m *messageInfo, field *protogen.Field) {
	goType, _ := fieldGoType(g, f, field)

	g.P(fmt.Sprintf("func (x *%s) Get%s() %s{", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("return x.%s", field.GoName))
	g.P("}")
	g.P(fmt.Sprintf("return %s", getDefaultValByKind(g, f, field, field.Desc.Kind())))
	g.P("}")

	g.P(fmt.Sprintf("func (x *%s) Set%s(v %s) {", m.GoIdent.GoName, field.GoName, goType))
	g.P("if x != nil {")
	g.P(fmt.Sprintf("x.%s = v", field.GoName))
	g.P(fmt.Sprintf("x.MarkDirty(x._idx_%s,1)", field.GoName))
	g.P("}")
	g.P("}")
}

func isJustStruct(field *protogen.Field) bool {
	return getFieldCategory(field) == StructField
}

func isStructSlice(field *protogen.Field) bool {
	return getFieldCategory(field) == StructSlice
}

func isStructMap(field *protogen.Field) bool {
	return getFieldCategory(field) == StructMap
}

func getFieldCategory(field *protogen.Field) StructFieldCategory {
	if field.Desc.IsMap() {
		if field.Desc.MapValue().Kind() == protoreflect.MessageKind {
			return StructMap
		} else {
			return BasicMap
		}
	}
	if field.Desc.IsList() {
		if field.Desc.Kind() == protoreflect.MessageKind {
			return StructSlice
		} else {
			return BasicSlice
		}
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		return StructField
	}
	return Basic
}

func getFieldsNoDeletedKey(m *messageInfo) []*protogen.Field {
	fs := make([]*protogen.Field, 0)
	for _, field := range m.Fields {
		if !strings.HasSuffix(field.GoName, "Deleted") {
			fs = append(fs, field)
		}
	}
	return fs
}

func getFieldsOnlyDeletedKey(m *messageInfo) []*protogen.Field {
	fs := make([]*protogen.Field, 0)
	for _, field := range m.Fields {
		if strings.HasSuffix(field.GoName, "Deleted") {
			fs = append(fs, field)
		}
	}
	return fs
}

func getDefaultValByKind(g *protogen.GeneratedFile, f *fileInfo, field *protogen.Field, kind protoreflect.Kind) string {
	switch kind {
	case protoreflect.Int64Kind, protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sint64Kind, protoreflect.Uint64Kind, protoreflect.Uint32Kind:
		return "0"
	case protoreflect.StringKind:
		return `""`
	case protoreflect.MessageKind:
		return "nil"
	case protoreflect.BoolKind:
		return "false"
	case protoreflect.DoubleKind, protoreflect.FloatKind:
		return "0"
	case protoreflect.EnumKind:
		val := field.Enum.Values[0]
		if val.GoIdent.GoImportPath == f.GoImportPath {
			return g.QualifiedGoIdent(val.GoIdent)
		} else {
			// If the enum value is declared in a different Go package,
			// reference it by number since the name may not be correct.
			// See https://github.com/golang/protobuf/issues/513.
			return g.QualifiedGoIdent(field.Enum.GoIdent) + "(" + strconv.FormatInt(int64(val.Desc.Number()), 10) + ")"
		}
	}
	return "nil"
}
