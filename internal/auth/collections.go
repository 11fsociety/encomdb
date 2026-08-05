package auth

import (
	pb "github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

const (
	CollectionApps  = "auth_apps"
	CollectionUsers = "auth_app_users"
)

// EnsureCollections creates the multi-tenant auth collections (idempotent).
//
//	auth_apps           — one row per Asmit's projects (Clerk-app equivalent)
//	auth_app_users      — per-app users; unique on (app_id, email)
func EnsureCollections(app *pb.PocketBase) error {
	if err := ensureAppsCollection(app); err != nil {
		return err
	}
	return ensureUsersCollection(app)
}

func ensureAppsCollection(app *pb.PocketBase) error {
	if existing, _ := app.FindCollectionByNameOrId(CollectionApps); existing != nil {
		return nil
	}
	col := core.NewBaseCollection(CollectionApps)
	col.Fields.Add(
		&core.TextField{Name: "name", Required: true},
		&core.TextField{Name: "slug", Required: true},
		&core.TextField{Name: "description"},
		&core.TextField{Name: "owner_id"},
		&core.TextField{Name: "publishable_key", Required: true},
		&core.TextField{Name: "secret_key_hash", Required: true},
		&core.TextField{Name: "jwt_secret", Required: true},
		&core.NumberField{Name: "session_ttl_hours"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)
	col.AddIndex("idx_auth_apps_slug", true, "slug", "")
	col.ListRule = nil
	col.ViewRule = nil
	col.CreateRule = nil
	col.UpdateRule = nil
	col.DeleteRule = nil
	return app.Save(col)
}

func ensureUsersCollection(app *pb.PocketBase) error {
	if existing, _ := app.FindCollectionByNameOrId(CollectionUsers); existing != nil {
		return nil
	}
	col := core.NewBaseCollection(CollectionUsers)
	col.Fields.Add(
		&core.TextField{Name: "app_id", Required: true},
		&core.TextField{Name: "email", Required: true},
		&core.TextField{Name: "password_hash", Required: true},
		&core.TextField{Name: "name"},
		&core.JSONField{Name: "metadata"},
		&core.BoolField{Name: "disabled"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)
	col.AddIndex("idx_auth_users_app_email", true, "app_id, email", "")
	col.ListRule = nil
	col.ViewRule = nil
	col.CreateRule = nil
	col.UpdateRule = nil
	col.DeleteRule = nil
	return app.Save(col)
}
