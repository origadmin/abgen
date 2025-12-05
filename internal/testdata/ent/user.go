package ent

import "time"

type Status string

type Role struct {
	ID   int64
	Name string
}

type User struct {
	ID        int64
	Username  string
	Password  string
	Salt      string
	Age       int
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
	Edges     UserEdges
}

type UserEdges struct {
	Roles []*Role
}
