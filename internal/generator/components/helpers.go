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
