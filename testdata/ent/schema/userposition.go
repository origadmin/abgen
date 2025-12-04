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

// UserPosition holds the schema definition for the UserPosition entity.
type UserPosition struct {
	ent.Schema
}

// Fields of the UserPosition.
func (UserPosition) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("user_id"),
		field.String("position_id"),
	}
}

// Indexes of the UserPosition.
func (UserPosition) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("position_id"),
		index.Fields("user_id", "position_id").
			Unique(),
	}
}

// Annotations of the UserPosition
func (UserPosition) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("sys_user_positions"),
		entsql.WithComments(true),
		schema.Comment("entity.user_position.table.comment"),
	}
}

// Edges of the UserPosition.
func (UserPosition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Field("user_id").
			Unique().
			Required(),
		edge.To("position", Position.Type).
			Field("position_id").
			Unique().
			Required(),
	}
}
