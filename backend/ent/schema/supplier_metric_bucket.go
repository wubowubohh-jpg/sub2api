package schema

import (
	"time"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type SupplierMetricBucket struct{ ent.Schema }
func (SupplierMetricBucket) Annotations() []schema.Annotation { return []schema.Annotation{entsql.Annotation{Table: "supplier_metric_buckets"}} }
func (SupplierMetricBucket) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id"), field.Time("bucket_start"), field.Enum("resolution").Values("5m", "1h", "1d"),
		field.Int64("request_count").Default(0), field.Int64("success_count").Default(0), field.Int64("total_tokens").Default(0),
		field.Int64("cache_read_tokens").Default(0), field.Int64("input_tokens").Default(0), field.Int64("duration_ms_sum").Default(0),
		field.Int64("first_token_ms_sum").Default(0), field.Int64("first_token_count").Default(0),
		field.Time("created_at").Default(time.Now).Immutable(), field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
func (SupplierMetricBucket) Indexes() []ent.Index { return []ent.Index{index.Fields("group_id", "resolution", "bucket_start").Unique(), index.Fields("resolution", "bucket_start")} }
