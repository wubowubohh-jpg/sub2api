package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"time"
)

type SupplierResourceRequest struct{ ent.Schema }

func (SupplierResourceRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "supplier_resource_requests"}}
}
func (SupplierResourceRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("supplier_id"), field.String("group_name").MaxLen(100).NotEmpty(), field.String("relay_name").MaxLen(100).NotEmpty(), field.String("relay_url").MaxLen(500).NotEmpty(),
		field.String("api_key_encrypted").Sensitive().NotEmpty(), field.String("model").MaxLen(200).Default("gpt-5.5"),
		field.Enum("status").Values("pending", "approved", "rejected").Default("pending"), field.Int64("reviewed_by").Optional().Nillable(), field.String("review_note").Default(""), field.Int64("group_id").Optional().Nillable(), field.Int64("account_id").Optional().Nillable(), field.Int64("monitor_id").Optional().Nillable(), field.Time("reviewed_at").Optional().Nillable(), field.Time("created_at").Default(time.Now).Immutable(),
	}
}
func (SupplierResourceRequest) Edges() []ent.Edge {
	return []ent.Edge{edge.From("supplier", Supplier.Type).Ref("resource_requests").Field("supplier_id").Unique().Required()}
}
func (SupplierResourceRequest) Indexes() []ent.Index {
	return []ent.Index{index.Fields("supplier_id", "status", "created_at")}
}
