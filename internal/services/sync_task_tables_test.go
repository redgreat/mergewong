package services

import (
	"testing"

	"github.com/redgreat/mergewong/internal/models"
)

func TestValidateTaskTables(t *testing.T) {
	tests := []struct {
		name   string
		tables []models.SyncTaskTable
		bad    bool
	}{
		{name: "multiple tables", tables: []models.SyncTaskTable{{SourceTable: "orders", TargetTable: "ods_orders"}, {SourceTable: "users", TargetTable: "ods_users"}}},
		{name: "empty", bad: true},
		{name: "duplicate source", tables: []models.SyncTaskTable{{SourceTable: "orders", TargetTable: "a"}, {SourceTable: "orders", TargetTable: "b"}}, bad: true},
		{name: "duplicate target", tables: []models.SyncTaskTable{{SourceTable: "a", TargetTable: "same"}, {SourceTable: "b", TargetTable: "same"}}, bad: true},
		{name: "unsafe name", tables: []models.SyncTaskTable{{SourceTable: "orders;drop", TargetTable: "orders"}}, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTaskTables(tt.tables)
			if tt.bad && err == nil {
				t.Fatal("expected error")
			}
			if !tt.bad && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
