package components

// GetHelperFunctionBodies returns a map of built-in helper function bodies.
func GetHelperFunctionBodies() map[string]string {
	return map[string]string{
		"ConvertStringToTime": `
func ConvertStringToTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
`,
		"ConvertStringToUUID": `
func ConvertStringToUUID(s string) uuid.UUID {
	u, _ := uuid.Parse(s)
	return u
}
`,
		"ConvertTimeToString": `
func ConvertTimeToString(t time.Time) string {
	return t.Format(time.RFC3339)
}
`,
		"ConvertUUIDToString": `
func ConvertUUIDToString(u uuid.UUID) string {
	return u.String()
}
`,
		"ConvertTimeToTimestamp": `
func ConvertTimeToTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
`,
		"ConvertTimestampToTime": `
func ConvertTimestampToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
`,
	}
}

// canDirectlyConvertPrimitives checks if two primitive types can be directly converted
func canDirectlyConvertPrimitives(sourceType, targetType string) bool {
	// 允许的数字类型转换
	numericTypes := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
	}

	// 如果都是数字类型，允许转换
	if numericTypes[sourceType] && numericTypes[targetType] {
		return true
	}

	// 相同类型允许转换
	if sourceType == targetType {
		return true
	}

	// 其他情况需要存根函数
	return false
}
