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

type SupplierLedger struct{ ent.Schema }

func (SupplierLedger) Annotations() []schema.Annotation { return []schema.Annotation{entsql.Annotation{Table: "supplier_ledgers"}} }
func (SupplierLedger) Fields() []ent.Field {
	decimal20 := map[string]string{dialect.Postgres: "decimal(20,10)"}
	decimal10 := map[string]string{dialect.Postgres: "decimal(10,4)"}
	return []ent.Field{
		field.Int64("supplier_id"), field.Int64("group_id"), field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("reversal_of_id").Optional().Nillable(),
		field.String("event_key").MaxLen(128).NotEmpty(),
		field.Enum("entry_type").Values("earning", "reversal", "release", "withdrawal", "withdrawal_refund"),
		field.Enum("bucket").Values("pending", "available", "frozen"),
		field.Float("base_rate").SchemaType(decimal10), field.Float("admin_adjustment").SchemaType(decimal10),
		field.Float("effective_rate").SchemaType(decimal10), field.Float("model_cost_usd").SchemaType(decimal20),
		field.Float("recharge_ratio").SchemaType(decimal20), field.Float("earning_usd").SchemaType(decimal20),
		field.Float("amount_cny").SchemaType(decimal20),
		field.Time("available_at").Optional().Nillable(), field.Time("created_at").Default(time.Now).Immutable(),
	}
}
func (SupplierLedger) Edges() []ent.Edge { return []ent.Edge{edge.From("supplier", Supplier.Type).Ref("ledger_entries").Field("supplier_id").Unique().Required()} }
func (SupplierLedger) Indexes() []ent.Index { return []ent.Index{index.Fields("event_key").Unique(), index.Fields("supplier_id", "bucket", "available_at"), index.Fields("usage_log_id")} }
