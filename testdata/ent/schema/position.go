// Package schema implements the functions, types, and interfaces for the module.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Position holds the schema definition for the Position entity.
type Position struct {
	ent.Schema
}

// Fields of the Position.
func (Position) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("name").
			Unique(),
		field.String("keyword").
			Unique(),
		field.String("department_id"),
	}
}

// Annotations of the Position.
func (Position) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("sys_positions"),
		entsql.WithComments(true),
		schema.Comment("entity.position.table.comment"),
	}
}

// Indexes of the Position.
func (Position) Indexes() []ent.Index {
	return []ent.Index{}
}

// Edges of the Position.
func (Position) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("department", Department.Type).
			Ref("positions").
			Field("department_id").
			Unique().
			Required(),
		edge.From("users", User.Type).
			Ref("positions").
			Through("user_positions", UserPosition.Type),
		edge.To("permissions", Permission.Type).
			Through("position_permissions", PositionPermission.Type),
	}
}
