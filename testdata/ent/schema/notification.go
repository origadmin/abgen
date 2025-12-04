// Package schema implements the functions, types, and interfaces for the module.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// Notification holds the schema definition for the Notification entity.
type Notification struct {
	ent.Schema
}

// Fields of the Notification.
func (Notification) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Comment("The ID of the permission.").
			Immutable().
			Unique(),
		field.String("subject"),
		field.String("content"),
		field.Int("status"),
		field.String("category_id"),
	}
}

// Annotations of the Notification.
func (Notification) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("msg_notifications"),
		entsql.WithComments(true),
		schema.Comment("entity.notification.table.comment"),
	}
}

// Indexes of the Notification.
func (Notification) Indexes() []ent.Index {
	return []ent.Index{}
}
