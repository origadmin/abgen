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

type PermissionResource struct {
	ent.Schema
}

func (PermissionResource) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("permission_id"),
		field.String("resource_id"),
	}
}

func (PermissionResource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("permission_id", "resource_id").
			Unique(),
	}
}

func (PermissionResource) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("sys_permission_resources"),
		entsql.WithComments(true),
		schema.Comment("entity.permission_resource.table.comment"),
	}
}

func (PermissionResource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("permission", Permission.Type).
			Field("permission_id").
			Unique().
			Required(),
		edge.To("resource", Resource.Type).
			Field("resource_id").
			Unique().
			Required(),
	}
}
