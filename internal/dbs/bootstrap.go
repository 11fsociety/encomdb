package dbs

import (
	"errors"
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

const (
	defaultAdminEmail    = "asmitdash44@gmail.com"
	defaultAdminPassword = "asmitdash44"
)

// EnsureDefaultAdmin creates or upserts a superuser with the configured
// credentials. Idempotent — if the record exists it just updates the password
// to the configured value (so restarting always yields a known-good login).
//
// Overrides:
//
//	ENCOMDB_ADMIN_EMAIL
//	ENCOMDB_ADMIN_PASSWORD
func EnsureDefaultAdmin(app *pocketbase.PocketBase) error {
	email := os.Getenv("ENCOMDB_ADMIN_EMAIL")
	if email == "" {
		email = defaultAdminEmail
	}
	password := os.Getenv("ENCOMDB_ADMIN_PASSWORD")
	if password == "" {
		password = defaultAdminPassword
	}

	col, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		return err
	}

	record, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, email)
	if err != nil {
		// Not found — create.
		record = core.NewRecord(col)
		record.SetEmail(email)
		record.SetPassword(password)
		if err := app.Save(record); err != nil {
			return err
		}
		log.Printf("[encomdb] created default superuser %s", email)
		return nil
	}

	// Existing — refresh password so restarts always yield a known login.
	record.SetPassword(password)
	if err := app.Save(record); err != nil {
		return err
	}
	log.Printf("[encomdb] refreshed default superuser %s", email)
	return nil
}

var _ = errors.New // reserved for future use
