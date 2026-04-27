package postgres_otelsql

import (
	"context"
	"testing"

	"github.com/XSAM/otelsql"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
)

func TestPostgresOtelSQLSpanConfiguration(t *testing.T) {
	t.Parallel()

	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1},
		SpanID:  trace.SpanID{1},
	})
	tracedCtx := trace.ContextWithSpanContext(context.Background(), parent)

	tests := []struct {
		name       string
		ctx        context.Context
		config     *Config
		method     otelsql.Method
		query      string
		wantSpan   bool
		wantName   string
		wantOp     string
		wantNoStmt bool
	}{
		{
			name:     "uses propagated trace and select operation",
			ctx:      tracedCtx,
			config:   DefaultConfig("postgres://user:pass@localhost:5432/app", "svc"),
			method:   otelsql.MethodConnQuery,
			query:    "select id from users where id = $1",
			wantSpan: true,
			wantName: "db.client.operation SELECT",
			wantOp:   "SELECT",
		},
		{
			name:     "does not create root span without propagated trace",
			ctx:      context.Background(),
			config:   DefaultConfig("postgres://user:pass@localhost:5432/app", "svc"),
			method:   otelsql.MethodConnExec,
			query:    "insert into users(id) values($1)",
			wantSpan: false,
			wantName: "db.client.operation INSERT",
			wantOp:   "INSERT",
		},
		{
			name: "respects tracing disabled",
			ctx:  tracedCtx,
			config: func() *Config {
				cfg := DefaultConfig("postgres://user:pass@localhost:5432/app", "svc")
				cfg.EnableTracing = false
				return cfg
			}(),
			method:   otelsql.MethodConnExec,
			query:    "update users set name = $1",
			wantSpan: false,
			wantName: "db.client.operation UPDATE",
			wantOp:   "UPDATE",
		},
		{
			name: "enables query text only with query logging",
			ctx:  tracedCtx,
			config: func() *Config {
				cfg := DefaultConfig("postgres://user:pass@localhost:5432/app", "svc")
				cfg.EnableQueryLogging = true
				return cfg
			}(),
			method:     otelsql.MethodConnExec,
			query:      "delete from users where id = $1",
			wantSpan:   true,
			wantName:   "db.client.operation DELETE",
			wantOp:     "DELETE",
			wantNoStmt: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			options := otelsqlOptions(tt.config)

			assert.NotEmpty(t, options)
			if tt.config.EnableTracing {
				assert.Equal(t, tt.wantSpan, propagatedTraceOnlySpanFilter(tt.ctx, tt.method, tt.query, nil))
			}
			assert.Equal(t, tt.wantName, formatDBSpanName(tt.ctx, tt.method, tt.query))
			assert.Equal(t, tt.wantOp, extractOperation(tt.query))
		})
	}
}

func TestExtractOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sql  string
		want string
	}{
		{name: "select", sql: " select * from users", want: "SELECT"},
		{name: "cte", sql: "WITH active AS (select 1) select * from active", want: "SELECT"},
		{name: "ddl", sql: "create table users(id text)", want: "DDL"},
		{name: "transaction", sql: "rollback", want: "TRANSACTION"},
		{name: "empty", sql: " \t\n", want: "UNKNOWN"},
		{name: "other", sql: "explain select 1", want: "OTHER"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, extractOperation(tt.sql))
		})
	}
}
