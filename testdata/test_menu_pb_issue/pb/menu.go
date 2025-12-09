package pb

// Menu represents the protobuf message type
type Menu struct {
	Id       int64  `json:"id"`
	Name     string `json:"name"`
	Icon     string `json:"icon"`
	Path     string `json:"path"`
	ParentId int64  `json:"parent_id"`
	Sort     int32  `json:"sort"`
	Status   int32  `json:"status"`
	CreatedAt string `json:"created_at"`
}