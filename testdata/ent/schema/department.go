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

// Department holds the schema definition for the Department domain.
type Department struct {
	ent.Schema
}

// Fields of the Department.
func (Department) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("name"),
		field.String("keyword").
			Unique(),
		field.Int("status"),
		field.String("parent_id").
			Optional(),
	}
}

// Indexes of the Department.
func (Department) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("keyword"),
		index.Fields("name"),
		index.Fields("status"),
	}
}

// Annotations of the Department.
func (Department) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("sys_departments"),
		entsql.WithComments(true),
		schema.Comment("entity.department.table.comment"),
	}
}

// Edges of the Department.
func (Department) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("users", User.Type).
			Ref("departments").
			Through("user_departments", UserDepartment.Type),
		edge.To("positions", Position.Type),
		edge.To("children", Department.Type).
			From("parent").Unique().Field("parent_id"),
	}
}
