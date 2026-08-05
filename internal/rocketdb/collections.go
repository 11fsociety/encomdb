package rocketdb

import (
	pb "github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

const collectionName = "rocketdb_projects"

// EnsureCollections creates the `rocketdb_projects` collection (idempotent).
func EnsureCollections(app *pb.PocketBase) error {
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
	col.AddIndex("idx_encom_dbs_name", true, "name", "")

	// Only superusers can read/write DBs via the collection API — our custom
	// endpoints do their own authorisation via api_key for the SQL runner.
	col.ListRule = nil
	col.ViewRule = nil
	col.CreateRule = nil
	col.UpdateRule = nil
	col.DeleteRule = nil

	return app.Save(col)
}
