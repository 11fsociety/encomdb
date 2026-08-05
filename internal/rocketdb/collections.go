package rocketdb

import (
	pb "github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

const collectionName = "rocketdb_projects"

// EnsureCollections creates the `rocketdb_projects` collection (idempotent).
// Also drops the legacy `encom_dbs` collection (renamed in v0.1.1) if present,
// so upgraders don't hit index-name collisions.
func EnsureCollections(app *pb.PocketBase) error {
	// Drop legacy collection if it exists (harmless if it doesn't).
	if legacy, _ := app.FindCollectionByNameOrId("encom_dbs"); legacy != nil {
		if err := app.Delete(legacy); err != nil {
			// Best-effort — log via error return so caller sees it, but don't panic
			// mid-boot if we can't clean up.
			_ = err
		}
	}

	if existing, _ := app.FindCollectionByNameOrId(collectionName); existing != nil {
		return nil
	}

	col := core.NewBaseCollection(collectionName)
	col.Fields.Add(
		&core.TextField{Name: "name", Required: true},
		&core.TextField{Name: "description"},
		&core.TextField{Name: "owner_id"},
		&core.TextField{Name: "api_key", Required: true},
		&core.NumberField{Name: "size_bytes"},
		&core.SelectField{
			Name:      "status",
			MaxSelect: 1,
			Values:    []string{"pending", "ready", "error"},
		},
		&core.TextField{Name: "error_message"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)
	col.AddIndex("idx_rocketdb_projects_name", true, "name", "")

	// Only superusers can read/write DBs via the collection API — our custom
	// endpoints do their own authorisation via api_key for the SQL runner.
	col.ListRule = nil
	col.ViewRule = nil
	col.CreateRule = nil
	col.UpdateRule = nil
	col.DeleteRule = nil

	return app.Save(col)
}
