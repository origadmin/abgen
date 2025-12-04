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

// PositionPermission holds the schema definition for the PositionPermission domain.
type PositionPermission struct {
	ent.Schema
}

// Fields of the PositionPermission.
func (PositionPermission) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("position_id"),
		field.String("permission_id"),
	}
}

// Indexes of the PositionPermission.
func (PositionPermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("permission_id"),
		index.Fields("position_id"),
		index.Fields("position_id", "permission_id").
			Unique(),
	}
}

// Annotations of the PositionPermission.
func (PositionPermission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("sys_position_permissions"),
		entsql.WithComments(true),
		schema.Comment("entity.position_permission.table.comment"),
	}
}

// Edges of the PositionPermission.
func (PositionPermission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("position", Position.Type).
			Field("position_id").
			Unique().
			Required(),
		edge.To("permission", Permission.Type).
			Field("permission_id").
			Unique().
			Required(),
	}
}
