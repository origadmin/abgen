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

// Resource holds the schema definition for the Resource domain.
type Resource struct {
	ent.Schema
}

// Fields of the Resource.
func (Resource) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("name"),
		field.String("keyword").
			Unique(),
		field.String("type"),
		field.Int("status"),
		field.String("path"),
		field.String("parent_id").
			Optional(),
	}
}

// Indexes of the Resource.
func (Resource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("parent_id"),
	}
}

// Annotations of the Menu.
func (Resource) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("sys_resources"),
		entsql.WithComments(true),
		schema.Comment("entity.resource.table.comment"),
	}
}

// Edges of the Resource.
func (Resource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Resource.Type),
		edge.From("parent", Resource.Type).
			Ref("children").
			Field("parent_id").
			Unique(),
		edge.From("permissions", Permission.Type).
			Ref("resources").
			Through("permission_resources", PermissionResource.Type),
	}
}
