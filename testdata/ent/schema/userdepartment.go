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

// UserDepartment holds the schema definition for the UserDepartment domain.
type UserDepartment struct {
	ent.Schema
}

// Fields of the UserDepartment.
func (UserDepartment) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("user_id"),
		field.String("department_id"),
	}
}

// Indexes of the UserDepartment.
func (UserDepartment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("department_id"),
		index.Fields("user_id", "department_id").
			Unique(),
	}
}

// Annotations of the UserDepartment.
func (UserDepartment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("sys_user_departments"),
		entsql.WithComments(true),
		schema.Comment("entity.user_department.table.comment"),
	}
}

// Edges of the UserDepartment.
func (UserDepartment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Field("user_id").
			Unique().
			Required(),
		edge.To("department", Department.Type).
			Field("department_id").
			Unique().
			Required(),
	}
}
