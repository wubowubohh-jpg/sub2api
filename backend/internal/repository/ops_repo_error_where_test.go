package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildOpsErrorLogsWhere_QueryUsesQualifiedColumns(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		Query: "ACCESS_DENIED",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "e.request_id ILIKE $") {
		t.Fatalf("where should include qualified request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.client_request_id ILIKE $") {
		t.Fatalf("where should include qualified client_request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.error_message ILIKE $") {
		t.Fatalf("where should include qualified error_message condition: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_UserQueryUsesExistsSubquery(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		UserQuery: "admin@",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "EXISTS (SELECT 1 FROM users u WHERE u.id = e.user_id AND u.email ILIKE $") {
		t.Fatalf("where should include EXISTS user email condition: %s", where)
	}
}

func TestOpsErrorLogsStatusCodeSemanticsByView(t *testing.T) {
	requestFilter := &service.OpsErrorLogFilter{
		StatusCodes: []int{499},
		SortBy:      "status_code",
		SortOrder:   "asc",
	}
	requestWhere, _ := buildOpsErrorLogsWhere(requestFilter)
	if !strings.Contains(requestWhere, "COALESCE(e.status_code, 0) = ANY($") {
		t.Fatalf("request view must filter by downstream status: %s", requestWhere)
	}
	if strings.Contains(requestWhere, "COALESCE(e.upstream_status_code, e.status_code, 0) = ANY($") {
		t.Fatalf("request view must not filter by upstream status: %s", requestWhere)
	}
	requestOrder := opsErrorLogsOrderBy(requestFilter)
	if requestOrder != "COALESCE(e.status_code, 0) ASC, e.id ASC" {
		t.Fatalf("request view status order = %q", requestOrder)
	}

	upstreamFilter := &service.OpsErrorLogFilter{
		IncludeRecoveredUpstream: true,
		StatusCodes:              []int{504},
		SortBy:                   "status_code",
		SortOrder:                "desc",
	}
	upstreamWhere, _ := buildOpsErrorLogsWhere(upstreamFilter)
	if !strings.Contains(upstreamWhere, "COALESCE(e.upstream_status_code, e.status_code, 0) = ANY($") {
		t.Fatalf("provider-health view must filter by upstream-first status: %s", upstreamWhere)
	}
	upstreamOrder := opsErrorLogsOrderBy(upstreamFilter)
	if upstreamOrder != "COALESCE(e.upstream_status_code, e.status_code, 0) DESC, e.id DESC" {
		t.Fatalf("provider-health view status order = %q", upstreamOrder)
	}
}
