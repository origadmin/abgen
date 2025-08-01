
{{define "time.Time:*timestamppb.Timestamp"}}
{{.Dst}} = timestamppb.New({{.Src}})
{{end}}

{{define "*timestamppb.Timestamp:time.Time"}}
{{.Dst}} = {{.Src}}.AsTime()
{{end}}

{{define "uuid.UUID:string"}}
{{.Dst}} = {{.Src}}.String()
{{end}}

{{define "string:uuid.UUID"}}
var err error
{{.Dst}}, err = uuid.Parse({{.Src}})
if err != nil {
    log.Printf("无法解析UUID字符串 %s: %v", {{.Src}}, err)
}
{{end}}

{{define "[]string:[]int"}}
{{if .Src}}
{{.Dst}} = make([]int, 0, len({{.Src}}))
for _, v := range {{.Src}} {
    if num, err := strconv.Atoi(v); err == nil {
        {{.Dst}} = append({{.Dst}}, num)
    }
}
{{end}}
{{end}}

{{define "[]int:[]string"}}
{{if .Src}}
{{.Dst}} = make([]string, len({{.Src}}))
for i, v := range {{.Src}} {
    {{.Dst}}[i] = strconv.Itoa(v)
}
{{end}}
{{end}}

{{define "map[string]interface{}:*structpb.Struct"}}
{{if .Src}}
var err error
{{.Dst}}, err = structpb.NewStruct({{.Src}})
if err != nil {
    log.Printf("转换结构化数据失败: %v", err)
}
{{end}}
{{end}}

{{define "*structpb.Struct:map[string]interface{}"}}
{{if .Src}}
{{.Dst}} = {{.Src}}.AsMap()
{{end}}
{{end}}

