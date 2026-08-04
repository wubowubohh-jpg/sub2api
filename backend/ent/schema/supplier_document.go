package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type SupplierDocument struct{ ent.Schema }

func (SupplierDocument) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "supplier_documents"}}
}

func (SupplierDocument) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("supplier_id"),
		field.String("storage_key").MaxLen(500).NotEmpty(),
		field.String("original_name").MaxLen(255).NotEmpty(),
		field.String("content_type").MaxLen(50).NotEmpty(),
		field.Int64("size_bytes"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (SupplierDocument) Edges() []ent.Edge {
	return []ent.Edge{edge.From("supplier", Supplier.Type).Ref("documents").Field("supplier_id").Unique().Required()}
}

func (SupplierDocument) Indexes() []ent.Index { return []ent.Index{index.Fields("supplier_id", "created_at")} }
