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

// User holds the schema definition for the User domain.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("uuid").
			Unique().
			Immutable(),
		field.String("username").
			Unique(),
		field.String("nickname"),
		field.String("email"),
		field.Int("status"),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("username"),
		index.Fields("email"),
		index.Fields("status"),
	}
}

// Annotations of the Role.
func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("sys_users"),
		entsql.WithComments(true),
		schema.Comment("entity.user.table.comment"),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("roles", Role.Type).
			Through("user_roles", UserRole.Type),
		edge.To("positions", Position.Type).
			Through("user_positions", UserPosition.Type),
		edge.To("departments", Department.Type).
			Through("user_departments", UserDepartment.Type),
	}
}

func (User) Hooks() []ent.Hook {
	return []ent.Hook{}
}

// SoftDelete schema to include control and time fields.
type SoftDelete struct {
	ent.Schema
}

// Fields of the Model.
func (SoftDelete) Fields() []ent.Field {
	return []ent.Field{
		field.Time("delete_time").
			Comment("delete_time.field.comment").
			Optional().
			Nillable(),
	}
}

// Indexes of the mixin.
func (SoftDelete) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("delete_time"),
	}
}
