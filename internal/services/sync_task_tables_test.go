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
		{name: "field mapping", tables: []models.SyncTaskTable{{SourceTable: "orders", TargetTable: "ods_orders", FieldMapping: models.FieldMapping{"buyer_id": "customer_id"}}}},
		{name: "empty", bad: true},
		{name: "duplicate source", tables: []models.SyncTaskTable{{SourceTable: "orders", TargetTable: "a"}, {SourceTable: "orders", TargetTable: "b"}}, bad: true},
		{name: "duplicate target", tables: []models.SyncTaskTable{{SourceTable: "a", TargetTable: "same"}, {SourceTable: "b", TargetTable: "same"}}, bad: true},
		{name: "unsafe name", tables: []models.SyncTaskTable{{SourceTable: "orders;drop", TargetTable: "orders"}}, bad: true},
		{name: "unsafe field mapping", tables: []models.SyncTaskTable{{SourceTable: "orders", TargetTable: "ods_orders", FieldMapping: models.FieldMapping{"buyer_id": "customer-id"}}}, bad: true},
		{name: "duplicate mapped target", tables: []models.SyncTaskTable{{SourceTable: "orders", TargetTable: "ods_orders", FieldMapping: models.FieldMapping{"buyer_id": "customer_id", "seller_id": "customer_id"}}}, bad: true},
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

func TestNormalizeFieldMapping(t *testing.T) {
	mapping, err := normalizeFieldMapping(models.FieldMapping{" buyer_id ": " customer_id ", "same": "same", "": ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapping["buyer_id"] != "customer_id" {
		t.Fatalf("expected trimmed mapping, got %#v", mapping)
	}
	if _, ok := mapping["same"]; ok {
		t.Fatalf("identity mapping should be omitted: %#v", mapping)
	}
}

func TestSyncSourceColumnsFiltersIgnoredFields(t *testing.T) {
	mapping := &models.SyncTaskTable{IgnoredFields: models.StringList{"IsSend"}}
	got := syncSourceColumns(mapping, []string{"ID", "Name", "IsSend", "UpdatedAt"})
	want := []string{"ID", "Name", "UpdatedAt"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}
