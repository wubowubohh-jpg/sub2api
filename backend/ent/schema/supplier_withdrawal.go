package schema

import (
	"time"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type SupplierWithdrawal struct{ ent.Schema }
func (SupplierWithdrawal) Annotations() []schema.Annotation { return []schema.Annotation{entsql.Annotation{Table: "supplier_withdrawals"}} }
func (SupplierWithdrawal) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("supplier_id"), field.String("request_no").MaxLen(64).NotEmpty(),
		field.Float("amount_cny").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Enum("method").Values("alipay", "wechat", "bank"),
		field.JSON("payout_snapshot", map[string]any{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Enum("status").Values("pending", "approved", "rejected", "paid").Default("pending"),
		field.Int64("reviewed_by").Optional().Nillable(), field.String("review_note").Default(""),
		field.Time("reviewed_at").Optional().Nillable(), field.String("payment_proof_key").MaxLen(500).Optional().Nillable(),
		field.Time("paid_at").Optional().Nillable(), field.Time("created_at").Default(time.Now).Immutable(), field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
func (SupplierWithdrawal) Edges() []ent.Edge { return []ent.Edge{edge.From("supplier", Supplier.Type).Ref("withdrawals").Field("supplier_id").Unique().Required()} }
func (SupplierWithdrawal) Indexes() []ent.Index { return []ent.Index{index.Fields("request_no").Unique(), index.Fields("supplier_id", "status", "created_at"), index.Fields("status", "created_at")} }
