package ent

// Menu represents the entity from the ent package
type Menu struct {
	ID   int64
	Name string
	Icon string
	Path string
	ParentID *int64
	Sort int32
	Status int32
	CreatedAt string
	UpdatedAt string
}