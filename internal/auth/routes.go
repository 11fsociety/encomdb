package auth

import (
	"errors"
	"net/http"
	"strings"

	pb "github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// RegisterRoutes wires the auth API onto the given group.
// Group is expected to be pre-namespaced (e.g. /api/auth).
func RegisterRoutes(g *router.RouterGroup[*core.RequestEvent], mgr *Manager, app *pb.PocketBase) {
	// Superuser-only: manage apps.
	g.GET("/apps", listApps(mgr)).Bind(apis.RequireSuperuserAuth())
	g.POST("/apps", createApp(mgr)).Bind(apis.RequireSuperuserAuth())
	g.GET("/apps/{id}", getApp(mgr)).Bind(apis.RequireSuperuserAuth())
	g.DELETE("/apps/{id}", deleteApp(mgr)).Bind(apis.RequireSuperuserAuth())
	g.POST("/apps/{id}/rotate-secret", rotateSecret(mgr)).Bind(apis.RequireSuperuserAuth())

	// Superuser-only: list + admin-delete users for an app.
	g.GET("/apps/{id}/users", listUsers(mgr)).Bind(apis.RequireSuperuserAuth())
	g.PATCH("/apps/{id}/users/{userId}", patchUser(mgr)).Bind(apis.RequireSuperuserAuth())
	g.DELETE("/apps/{id}/users/{userId}", deleteUser(mgr)).Bind(apis.RequireSuperuserAuth())

	// Public app endpoints — anyone with the publishable_key can signup/login.
	// They target apps by slug OR id (slug is the friendly URL).
	g.POST("/apps/{id}/signup", signup(mgr))
	g.POST("/apps/{id}/login", login(mgr))
	g.GET("/apps/{id}/me", me(mgr))
}

// --- app CRUD (superuser) ---

type createAppReq struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func createApp(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		var req createAppReq
		if err := e.BindBody(&req); err != nil {
			return apis.NewBadRequestError("invalid json", err)
		}
		ownerID := ""
		if a := e.Auth; a != nil {
			ownerID = a.Id
		}
		out, secret, err := mgr.CreateApp(req.Name, req.Slug, req.Description, ownerID)
		if err != nil {
			if errors.Is(err, ErrInvalidSlug) || strings.Contains(err.Error(), "already taken") {
				return apis.NewBadRequestError(err.Error(), err)
			}
			return apis.NewInternalServerError(err.Error(), err)
		}
		out.SecretKey = secret // one-shot: last time we return the plaintext secret
		return e.JSON(http.StatusCreated, out)
	}
}

func listApps(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		apps, err := mgr.ListApps()
		if err != nil {
			return apis.NewInternalServerError(err.Error(), err)
		}
		return e.JSON(http.StatusOK, apps)
	}
}

func getApp(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		rec, err := mgr.GetApp(e.Request.PathValue("id"))
		if err != nil {
			return apis.NewNotFoundError("app not found", err)
		}
		return e.JSON(http.StatusOK, mgr.appFromRecord(rec))
	}
}

func deleteApp(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if err := mgr.DeleteApp(e.Request.PathValue("id")); err != nil {
			if errors.Is(err, ErrAppNotFound) {
				return apis.NewNotFoundError("app not found", err)
			}
			return apis.NewInternalServerError(err.Error(), err)
		}
		return e.JSON(http.StatusOK, map[string]any{"deleted": true})
	}
}

func rotateSecret(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		secret, err := mgr.RotateSecret(e.Request.PathValue("id"))
		if err != nil {
			if errors.Is(err, ErrAppNotFound) {
				return apis.NewNotFoundError("app not found", err)
			}
			return apis.NewInternalServerError(err.Error(), err)
		}
		return e.JSON(http.StatusOK, map[string]any{"secret_key": secret})
	}
}

// --- user admin (superuser) ---

func listUsers(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		appRec, err := mgr.GetApp(e.Request.PathValue("id"))
		if err != nil {
			return apis.NewNotFoundError("app not found", err)
		}
		users, err := mgr.ListUsers(appRec, 500)
		if err != nil {
			return apis.NewInternalServerError(err.Error(), err)
		}
		return e.JSON(http.StatusOK, users)
	}
}

type patchUserReq struct {
	Disabled *bool `json:"disabled,omitempty"`
}

func patchUser(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		appRec, err := mgr.GetApp(e.Request.PathValue("id"))
		if err != nil {
			return apis.NewNotFoundError("app not found", err)
		}
		var req patchUserReq
		if err := e.BindBody(&req); err != nil {
			return apis.NewBadRequestError("invalid json", err)
		}
		if req.Disabled != nil {
			if err := mgr.SetDisabled(appRec, e.Request.PathValue("userId"), *req.Disabled); err != nil {
				return apis.NewNotFoundError("user not found", err)
			}
		}
		return e.JSON(http.StatusOK, map[string]any{"ok": true})
	}
}

func deleteUser(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		appRec, err := mgr.GetApp(e.Request.PathValue("id"))
		if err != nil {
			return apis.NewNotFoundError("app not found", err)
		}
		if err := mgr.DeleteUser(appRec, e.Request.PathValue("userId")); err != nil {
			return apis.NewNotFoundError("user not found", err)
		}
		return e.JSON(http.StatusOK, map[string]any{"deleted": true})
	}
}

// --- public app endpoints ---

type signupReq struct {
	Email    string         `json:"email"`
	Password string         `json:"password"`
	Name     string         `json:"name"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func signup(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		appRec, err := mgr.GetApp(e.Request.PathValue("id"))
		if err != nil {
			return apis.NewNotFoundError("app not found", err)
		}
		var req signupReq
		if err := e.BindBody(&req); err != nil {
			return apis.NewBadRequestError("invalid json", err)
		}
		u, sess, err := mgr.Signup(appRec, req.Email, req.Password, req.Name, req.Metadata)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidEmail),
				errors.Is(err, ErrWeakPassword),
				errors.Is(err, ErrDuplicate):
				return apis.NewBadRequestError(err.Error(), err)
			default:
				return apis.NewInternalServerError(err.Error(), err)
			}
		}
		return e.JSON(http.StatusCreated, map[string]any{"user": u, "session": sess})
	}
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func login(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		appRec, err := mgr.GetApp(e.Request.PathValue("id"))
		if err != nil {
			return apis.NewNotFoundError("app not found", err)
		}
		var req loginReq
		if err := e.BindBody(&req); err != nil {
			return apis.NewBadRequestError("invalid json", err)
		}
		sess, err := mgr.Login(appRec, req.Email, req.Password)
		if err != nil {
			return apis.NewUnauthorizedError(err.Error(), err)
		}
		return e.JSON(http.StatusOK, sess)
	}
}

func me(mgr *Manager) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		appRec, err := mgr.GetApp(e.Request.PathValue("id"))
		if err != nil {
			return apis.NewNotFoundError("app not found", err)
		}
		auth := e.Request.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			return apis.NewUnauthorizedError("bearer token required", nil)
		}
		token := strings.TrimSpace(auth[7:])
		verifiedApp, userRec, err := mgr.VerifyJWT(token)
		if err != nil {
			return apis.NewUnauthorizedError(err.Error(), err)
		}
		if verifiedApp.Id != appRec.Id {
			return apis.NewUnauthorizedError("token issued for a different app", nil)
		}
		return e.JSON(http.StatusOK, mgr.userFromRecord(userRec))
	}
}
