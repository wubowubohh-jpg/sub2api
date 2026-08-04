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

// Supplier stores the supplier capability attached to a normal user account.
type Supplier struct{ ent.Schema }

func (Supplier) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "suppliers"}}
}

func (Supplier) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("name").MaxLen(100).NotEmpty(),
		field.String("relay_url").MaxLen(500).NotEmpty(),
		field.String("application_note").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.Enum("status").Values("pending", "approved", "rejected", "frozen").Default("pending"),
		field.Int64("reviewed_by").Optional().Nillable(),
		field.String("review_note").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.Time("reviewed_at").Optional().Nillable(),
		field.String("freeze_reason").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.Float("pending_balance_cny").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(0),
		field.Float("available_balance_cny").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(0),
		field.Float("frozen_balance_cny").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(0),
		field.JSON("payout_profile", map[string]any{}).Default(map[string]any{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Supplier) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("supplier").Field("user_id").Unique().Required(),
		edge.To("documents", SupplierDocument.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("groups", Group.Type),
		edge.To("accounts", Account.Type),
		edge.To("ledger_entries", SupplierLedger.Type),
		edge.To("withdrawals", SupplierWithdrawal.Type),
		edge.To("resource_requests", SupplierResourceRequest.Type),
	}
}

func (Supplier) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id").Unique(), index.Fields("status"), index.Fields("created_at")}
}
