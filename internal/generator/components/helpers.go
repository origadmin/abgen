package components

import "github.com/origadmin/abgen/internal/model"

const (
	timePkg        = "time"
	uuidPkg        = "github.com/google/uuid"
	timestamppbPkg = "google.golang.org/protobuf/types/known/timestamppb"
	wrapperspbPkg  = "google.golang.org/protobuf/types/known/wrapperspb"
	durationpbPkg  = "google.golang.org/protobuf/types/known/durationpb"
)

// GetBuiltInHelpers returns a list of all built-in helper functions.
func GetBuiltInHelpers() []model.Helper {
	return []model.Helper{
		// time.Time <-> string
		{
			Name:         "ConvertStringToTime",
			SourceType:   "string",
			TargetType:   "time.Time",
			Dependencies: []string{timePkg},
			Body: `
func ConvertStringToTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}`,
		},
		{
			Name:         "ConvertTimeToString",
			SourceType:   "time.Time",
			TargetType:   "string",
			Dependencies: []string{timePkg},
			Body: `
func ConvertTimeToString(t time.Time) string {
	return t.Format(time.RFC3339)
}`,
		},
		// uuid.UUID <-> string
		{
			Name:         "ConvertStringToUUID",
			SourceType:   "string",
			TargetType:   "github.com/google/uuid.UUID",
			Dependencies: []string{uuidPkg},
			Body: `
func ConvertStringToUUID(s string) uuid.UUID {
	u, _ := uuid.Parse(s)
	return u
}`,
		},
		{
			Name:         "ConvertUUIDToString",
			SourceType:   "github.com/google/uuid.UUID",
			TargetType:   "string",
			Dependencies: []string{uuidPkg},
			Body: `
func ConvertUUIDToString(u uuid.UUID) string {
	return u.String()
}`,
		},
		// time.Time <-> *timestamppb.Timestamp
		{
			Name:         "ConvertTimeToTimestamp",
			SourceType:   "time.Time",
			TargetType:   "*google.golang.org/protobuf/types/known/timestamppb.Timestamp",
			Dependencies: []string{timePkg, timestamppbPkg},
			Body: `
func ConvertTimeToTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}`,
		},
		{
			Name:         "ConvertTimestampToTime",
			SourceType:   "*google.golang.org/protobuf/types/known/timestamppb.Timestamp",
			TargetType:   "time.Time",
			Dependencies: []string{timePkg, timestamppbPkg},
			Body: `
func ConvertTimestampToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}`,
		},
		// string <-> *wrapperspb.StringValue
		{
			Name:         "ConvertStringToStringValue",
			SourceType:   "string",
			TargetType:   "*google.golang.org/protobuf/types/known/wrapperspb.StringValue",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertStringToStringValue(s string) *wrapperspb.StringValue {
	return wrapperspb.String(s)
}`,
		},
		{
			Name:         "ConvertStringValueToString",
			SourceType:   "*google.golang.org/protobuf/types/known/wrapperspb.StringValue",
			TargetType:   "string",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertStringValueToString(v *wrapperspb.StringValue) string {
	if v == nil {
		return ""
	}
	return v.GetValue()
}`,
		},
		// int32 <-> *wrapperspb.Int32Value
		{
			Name:         "ConvertInt32ToInt32Value",
			SourceType:   "int32",
			TargetType:   "*google.golang.org/protobuf/types/known/wrapperspb.Int32Value",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertInt32ToInt32Value(i int32) *wrapperspb.Int32Value {
	return wrapperspb.Int32(i)
}`,
		},
		{
			Name:         "ConvertInt32ValueToInt32",
			SourceType:   "*google.golang.org/protobuf/types/known/wrapperspb.Int32Value",
			TargetType:   "int32",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertInt32ValueToInt32(v *wrapperspb.Int32Value) int32 {
	if v == nil {
		return 0
	}
	return v.GetValue()
}`,
		},
		// int64 <-> *wrapperspb.Int64Value
		{
			Name:         "ConvertInt64ToInt64Value",
			SourceType:   "int64",
			TargetType:   "*google.golang.org/protobuf/types/known/wrapperspb.Int64Value",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertInt64ToInt64Value(i int64) *wrapperspb.Int64Value {
	return wrapperspb.Int64(i)
}`,
		},
		{
			Name:         "ConvertInt64ValueToInt64",
			SourceType:   "*google.golang.org/protobuf/types/known/wrapperspb.Int64Value",
			TargetType:   "int64",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertInt64ValueToInt64(v *wrapperspb.Int64Value) int64 {
	if v == nil {
		return 0
	}
	return v.GetValue()
}`,
		},
		// uint32 <-> *wrapperspb.UInt32Value
		{
			Name:         "ConvertUInt32ToUInt32Value",
			SourceType:   "uint32",
			TargetType:   "*google.golang.org/protobuf/types/known/wrapperspb.UInt32Value",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertUInt32ToUInt32Value(i uint32) *wrapperspb.UInt32Value {
	return wrapperspb.UInt32(i)
}`,
		},
		{
			Name:         "ConvertUInt32ValueToUInt32",
			SourceType:   "*google.golang.org/protobuf/types/known/wrapperspb.UInt32Value",
			TargetType:   "uint32",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertUInt32ValueToUInt32(v *wrapperspb.UInt32Value) uint32 {
	if v == nil {
		return 0
	}
	return v.GetValue()
}`,
		},
		// uint64 <-> *wrapperspb.UInt64Value
		{
			Name:         "ConvertUInt64ToUInt64Value",
			SourceType:   "uint64",
			TargetType:   "*google.golang.org/protobuf/types/known/wrapperspb.UInt64Value",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertUInt64ToUInt64Value(i uint64) *wrapperspb.UInt64Value {
	return wrapperspb.UInt64(i)
}`,
		},
		{
			Name:         "ConvertUInt64ValueToUInt64",
			SourceType:   "*google.golang.org/protobuf/types/known/wrapperspb.UInt64Value",
			TargetType:   "uint64",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertUInt64ValueToUInt64(v *wrapperspb.UInt64Value) uint64 {
	if v == nil {
		return 0
	}
	return v.GetValue()
}`,
		},
		// float32 <-> *wrapperspb.FloatValue
		{
			Name:         "ConvertFloatToFloatValue",
			SourceType:   "float32",
			TargetType:   "*google.golang.org/protobuf/types/known/wrapperspb.FloatValue",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertFloatToFloatValue(f float32) *wrapperspb.FloatValue {
	return wrapperspb.Float(f)
}`,
		},
		{
			Name:         "ConvertFloatValueToFloat",
			SourceType:   "*google.golang.org/protobuf/types/known/wrapperspb.FloatValue",
			TargetType:   "float32",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertFloatValueToFloat(v *wrapperspb.FloatValue) float32 {
	if v == nil {
		return 0.0
	}
	return v.GetValue()
}`,
		},
		// float64 <-> *wrapperspb.DoubleValue
		{
			Name:         "ConvertDoubleToDoubleValue",
			SourceType:   "float64",
			TargetType:   "*google.golang.org/protobuf/types/known/wrapperspb.DoubleValue",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertDoubleToDoubleValue(d float64) *wrapperspb.DoubleValue {
	return wrapperspb.Double(d)
}`,
		},
		{
			Name:         "ConvertDoubleValueToDouble",
			SourceType:   "*google.golang.org/protobuf/types/known/wrapperspb.DoubleValue",
			TargetType:   "float64",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertDoubleValueToDouble(v *wrapperspb.DoubleValue) float64 {
	if v == nil {
		return 0.0
	}
	return v.GetValue()
}`,
		},
		// bool <-> *wrapperspb.BoolValue
		{
			Name:         "ConvertBoolToBoolValue",
			SourceType:   "bool",
			TargetType:   "*google.golang.org/protobuf/types/known/wrapperspb.BoolValue",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertBoolToBoolValue(b bool) *wrapperspb.BoolValue {
	return wrapperspb.Bool(b)
}`,
		},
		{
			Name:         "ConvertBoolValueToBool",
			SourceType:   "*google.golang.org/protobuf/types/known/wrapperspb.BoolValue",
			TargetType:   "bool",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertBoolValueToBool(v *wrapperspb.BoolValue) bool {
	if v == nil {
		return false
	}
	return v.GetValue()
}`,
		},
		// []byte <-> *wrapperspb.BytesValue
		{
			Name:         "ConvertBytesToBytesValue",
			SourceType:   "[]byte",
			TargetType:   "*google.golang.org/protobuf/types/known/wrapperspb.BytesValue",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertBytesToBytesValue(b []byte) *wrapperspb.BytesValue {
	return wrapperspb.Bytes(b)
}`,
		},
		{
			Name:         "ConvertBytesValueToBytes",
			SourceType:   "*google.golang.org/protobuf/types/known/wrapperspb.BytesValue",
			TargetType:   "[]byte",
			Dependencies: []string{wrapperspbPkg},
			Body: `
func ConvertBytesValueToBytes(v *wrapperspb.BytesValue) []byte {
	if v == nil {
		return nil
	}
	return v.GetValue()
}`,
		},
		// time.Duration <-> *durationpb.Duration
		{
			Name:         "ConvertDurationToDurationpb",
			SourceType:   "time.Duration",
			TargetType:   "*google.golang.org/protobuf/types/known/durationpb.Duration",
			Dependencies: []string{timePkg, durationpbPkg},
			Body: `
func ConvertDurationToDurationpb(d time.Duration) *durationpb.Duration {
	return durationpb.New(d)
}`,
		},
		{
			Name:         "ConvertDurationpbToDuration",
			SourceType:   "*google.golang.org/protobuf/types/known/durationpb.Duration",
			TargetType:   "time.Duration",
			Dependencies: []string{timePkg, durationpbPkg},
			Body: `
func ConvertDurationpbToDuration(d *durationpb.Duration) time.Duration {
	if d == nil {
		return 0
	}
	return d.AsDuration()
}`,
		},
	}
}
