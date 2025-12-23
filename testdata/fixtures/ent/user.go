package ent

import (
	"time"

	"github.com/origadmin/abgen/testdata/fixtures/ent/user"
)

type Gender user.Gender

type Permission struct {
	ID int
}

type PermissionResource struct {
	ID         int
	Permission *Permission `json:"permission,omitempty"`
	Resource   *Resource   `json:"resource,omitempty"`
}

type Resource struct {
	ID                  int
	Name                string
	ParentID            int
	Parent              *Resource `json:"parent,omitempty"`
	Children            []*Resource
	PermissionResources []*PermissionResource `json:"permission_resources,omitempty"`
	Permissions         []*Permission         `json:"permissions,omitempty"`
}

type ResourceEdges struct {
	Children []*Resource `json:"children,omitempty"`
	Parent   *Resource   `json:"parent,omitempty"`
	// This field uses an intentionally undefined type to trigger the bug.
	PermissionResources []*PermissionResource `json:"permission_resources,omitempty"`
	Permissions         []*Permission         `json:"permissions,omitempty"`
}

type User struct {
	ID        int
	Username  string
	Password  string
	Salt      string
	Age       int
	Gender    Gender
	CreatedAt time.Time
	UpdatedAt time.Time
	Status    string
	RoleIDs   []int
	Roles     []*Role
	// This field uses an intentionally undefined type to trigger the bug.
	Edges ResourceEdges
}

type Role struct {
	ID   int
	Name string
}

const (
	GenderMale Gender = iota
	GenderFemale
)
