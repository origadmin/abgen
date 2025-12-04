// Package schema implements the functions, types, and interfaces for the module.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserRole holds the schema definition for the UserRole domain.
type UserRole struct {
	ent.Schema
}

// Fields of the UserRole.
func (UserRole) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("user_id"),
		field.String("role_id"),
	}
}

// Indexes of the UserRole.
func (UserRole) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("role_id"),
		index.Fields("user_id", "role_id").
			Unique(),
	}
}

// Annotations of the UserRole.
func (UserRole) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("sys_user_roles"),
		entsql.WithComments(true),
		schema.Comment("entity.(1).(2).(3)"),
	}
}

// Edges of the UserRole.
func (UserRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Field("user_id").
			Required().
			Unique(),
		edge.To("role", Role.Type).
			Field("role_id").
			Required().
			Unique(),
	}
}
