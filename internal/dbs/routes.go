package dbs

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// RegisterRoutes wires the Encom API onto the given group.
// Expected to be pre-namespaced (e.g. /api/encom).
func RegisterRoutes(g *router.RouterGroup[*core.RequestEvent], mgr *Manager, app *pocketbase.PocketBase) {
	// Admin-only: manage databases.
	g.GET("/dbs", listDBs(mgr, app)).Bind(apis.RequireSuperuserAuth())
	g.POST("/dbs", createDB(mgr, app)).Bind(apis.RequireSuperuserAuth())
	g.GET("/dbs/{name}", getDB(mgr, app)).Bind(apis.RequireSuperuserAuth())
	g.DELETE("/dbs/{name}", deleteDB(mgr, app)).Bind(apis.RequireSuperuserAuth())

	// Public URL info (superuser only — reveals the tunnel host).
	g.GET("/tunnel", tunnelInfo(mgr)).Bind(apis.RequireSuperuserAuth())

	// SQL execution — authorised by the DB's api_key.
	g.POST("/dbs/{name}/sql", runSQL(mgr, app))
}

func tunnelInfo(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		return e.JSON(200, map[string]any{
			"public_host": mgr.PublicHost(),
			"tunnel_url":  mgr.TunnelURL(),
		})
	}
}

type createReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func createDB(mgr *Manager, app *pocketbase.PocketBase) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		var req createReq
		if err := e.BindBody(&req); err != nil {
			return apis.NewBadRequestError("invalid json", err)
		}
		req.Name = strings.TrimSpace(strings.ToLower(req.Name))
		if !ValidName(req.Name) {
			return apis.NewBadRequestError("name must be lowercase letters/digits/_/-, 3-40 chars", nil)
		}
		ownerID := ""
		if auth := e.Auth; auth != nil {
			ownerID = auth.Id
		}
		record, err := mgr.Create(e.Request.Context(), req.Name, req.Description, ownerID)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				return apis.NewApiError(http.StatusConflict, err.Error(), err)
			}
			return apis.NewBadRequestError(err.Error(), err)
		}
		return e.JSON(http.StatusCreated, mgr.ConnectionInfo(record))
	}
}

func listDBs(mgr *Manager, app *pocketbase.PocketBase) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		records, err := app.FindRecordsByFilter(collectionName, "", "-created", 200, 0)
		if err != nil {
			return apis.NewInternalServerError(err.Error(), err)
		}
		out := make([]ConnectionInfo, 0, len(records))
		for _, r := range records {
			out = append(out, mgr.ConnectionInfo(r))
		}
		return e.JSON(http.StatusOK, out)
	}
}

func getDB(mgr *Manager, app *pocketbase.PocketBase) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		name := e.Request.PathValue("name")
		record, err := app.FindFirstRecordByFilter(collectionName, "name = {:n}", map[string]any{"n": name})
		if err != nil {
			return apis.NewNotFoundError("database not found", err)
		}
		return e.JSON(http.StatusOK, mgr.ConnectionInfo(record))
	}
}

func deleteDB(mgr *Manager, app *pocketbase.PocketBase) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		name := e.Request.PathValue("name")
		if err := mgr.Delete(e.Request.Context(), name); err != nil {
			if errors.Is(err, ErrNotFound) {
				return apis.NewNotFoundError("database not found", err)
			}
			return apis.NewInternalServerError(err.Error(), err)
		}
		return e.JSON(http.StatusOK, map[string]any{"deleted": true})
	}
}

type sqlReq struct {
	SQL  string `json:"sql"`
	Args []any  `json:"args,omitempty"`
}

type sqlResp struct {
	Columns      []string `json:"columns,omitempty"`
	Rows         [][]any  `json:"rows,omitempty"`
	RowsAffected int64    `json:"rows_affected,omitempty"`
	Duration     int64    `json:"duration_ms"`
}

const maxRows = 10000

func runSQL(mgr *Manager, app *pocketbase.PocketBase) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		name := e.Request.PathValue("name")
		record, err := app.FindFirstRecordByFilter(collectionName, "name = {:n}", map[string]any{"n": name})
		if err != nil {
			return apis.NewNotFoundError("database not found", err)
		}

		auth := e.Request.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			return apis.NewUnauthorizedError("bearer token required", nil)
		}
		token := strings.TrimSpace(auth[7:])
		if token == "" || token != record.GetString("api_key") {
			return apis.NewUnauthorizedError("invalid api key", nil)
		}

		var req sqlReq
		if err := e.BindBody(&req); err != nil {
			return apis.NewBadRequestError("invalid json", err)
		}
		req.SQL = strings.TrimSpace(req.SQL)
		if req.SQL == "" {
			return apis.NewBadRequestError("sql required", nil)
		}

		db, err := mgr.Open(name)
		if err != nil {
			return apis.NewInternalServerError(err.Error(), err)
		}

		ctx := e.Request.Context()
		start := timeNowMs()
		trimmed := strings.ToUpper(strings.TrimSpace(req.SQL))
		isRead := strings.HasPrefix(trimmed, "SELECT") || strings.HasPrefix(trimmed, "WITH") || strings.HasPrefix(trimmed, "PRAGMA") || strings.HasPrefix(trimmed, "EXPLAIN")

		if !isRead {
			res, err := db.ExecContext(ctx, req.SQL, req.Args...)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), err)
			}
			n, _ := res.RowsAffected()
			return e.JSON(http.StatusOK, sqlResp{RowsAffected: n, Duration: timeNowMs() - start})
		}

		rows, err := db.QueryContext(ctx, req.SQL, req.Args...)
		if err != nil {
			return apis.NewBadRequestError(err.Error(), err)
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			return apis.NewInternalServerError(err.Error(), err)
		}
		out := sqlResp{Columns: cols, Rows: make([][]any, 0, 64)}
		for rows.Next() {
			if len(out.Rows) >= maxRows {
				return apis.NewBadRequestError(fmt.Sprintf("result set exceeds %d rows", maxRows), nil)
			}
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return apis.NewInternalServerError(err.Error(), err)
			}
			for i, v := range vals {
				if b, ok := v.([]byte); ok {
					vals[i] = string(b)
				}
			}
			out.Rows = append(out.Rows, vals)
		}
		if err := rows.Err(); err != nil {
			return apis.NewInternalServerError(err.Error(), err)
		}
		out.Duration = timeNowMs() - start
		return e.JSON(http.StatusOK, out)
	}
}
