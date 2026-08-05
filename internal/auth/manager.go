package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	pb "github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAppNotFound  = errors.New("app not found")
	ErrUserNotFound = errors.New("user not found")
	ErrBadPassword  = errors.New("invalid credentials")
	ErrDisabled     = errors.New("user disabled")
	ErrDuplicate    = errors.New("email already registered for this app")
	ErrInvalidSlug  = errors.New("slug must be lowercase letters/digits/_/-, 3-40 chars")
	ErrInvalidEmail = errors.New("email is invalid")
	ErrWeakPassword = errors.New("password must be at least 8 characters")
)

var slugRE = regexp.MustCompile(`^[a-z0-9_-]{3,40}$`)
var emailRE = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

const (
	defaultSessionTTLHours = 24 * 7
	bcryptCost             = 10
)

type Manager struct {
	app *pb.PocketBase
}

func NewManager(app *pb.PocketBase) *Manager {
	return &Manager{app: app}
}

type App struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	Description     string `json:"description"`
	PublishableKey  string `json:"publishable_key"`
	SecretKey       string `json:"secret_key,omitempty"` // populated only on create
	SessionTTLHours int    `json:"session_ttl_hours"`
	Created         string `json:"created"`
}

type User struct {
	ID       string         `json:"id"`
	AppID    string         `json:"app_id"`
	Email    string         `json:"email"`
	Name     string         `json:"name"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Disabled bool           `json:"disabled"`
	Created  string         `json:"created"`
}

type Session struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	User      User   `json:"user"`
}

func ValidSlug(s string) bool { return slugRE.MatchString(s) }
func ValidEmail(s string) bool {
	return len(s) < 200 && emailRE.MatchString(strings.ToLower(strings.TrimSpace(s)))
}

// --- Apps ---

// CreateApp registers a new tenant app.
func (m *Manager) CreateApp(name, slug, description, ownerID string) (App, string, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	name = strings.TrimSpace(name)
	if name == "" {
		return App{}, "", errors.New("name required")
	}
	if !ValidSlug(slug) {
		return App{}, "", ErrInvalidSlug
	}

	existing, _ := m.app.FindFirstRecordByFilter(CollectionApps, "slug = {:s}", map[string]any{"s": slug})
	if existing != nil {
		return App{}, "", fmt.Errorf("app slug %q already taken", slug)
	}

	col, err := m.app.FindCollectionByNameOrId(CollectionApps)
	if err != nil {
		return App{}, "", err
	}

	pk := "pk_" + randomHex(16)
	sk := "sk_" + randomHex(24)
	skHash, err := bcrypt.GenerateFromPassword([]byte(sk), bcryptCost)
	if err != nil {
		return App{}, "", err
	}
	jwtSecret := randomHex(32)

	rec := core.NewRecord(col)
	rec.Set("name", name)
	rec.Set("slug", slug)
	rec.Set("description", description)
	rec.Set("owner_id", ownerID)
	rec.Set("publishable_key", pk)
	rec.Set("secret_key_hash", string(skHash))
	rec.Set("jwt_secret", jwtSecret)
	rec.Set("session_ttl_hours", defaultSessionTTLHours)

	if err := m.app.Save(rec); err != nil {
		return App{}, "", err
	}

	out := m.appFromRecord(rec)
	out.SecretKey = sk
	return out, sk, nil
}

func (m *Manager) ListApps() ([]App, error) {
	records, err := m.app.FindRecordsByFilter(CollectionApps, "", "-created", 500, 0)
	if err != nil {
		return nil, err
	}
	out := make([]App, 0, len(records))
	for _, r := range records {
		out = append(out, m.appFromRecord(r))
	}
	return out, nil
}

func (m *Manager) GetApp(idOrSlug string) (*core.Record, error) {
	if rec, err := m.app.FindRecordById(CollectionApps, idOrSlug); err == nil {
		return rec, nil
	}
	rec, err := m.app.FindFirstRecordByFilter(CollectionApps, "slug = {:s}", map[string]any{"s": idOrSlug})
	if err != nil || rec == nil {
		return nil, ErrAppNotFound
	}
	return rec, nil
}

func (m *Manager) DeleteApp(idOrSlug string) error {
	rec, err := m.GetApp(idOrSlug)
	if err != nil {
		return err
	}
	// Also cascade-delete users for this app.
	users, _ := m.app.FindRecordsByFilter(CollectionUsers, "app_id = {:a}", "", 0, 0, map[string]any{"a": rec.Id})
	for _, u := range users {
		_ = m.app.Delete(u)
	}
	return m.app.Delete(rec)
}

// RotateSecret regenerates a new secret_key for the app.
func (m *Manager) RotateSecret(idOrSlug string) (string, error) {
	rec, err := m.GetApp(idOrSlug)
	if err != nil {
		return "", err
	}
	sk := "sk_" + randomHex(24)
	skHash, err := bcrypt.GenerateFromPassword([]byte(sk), bcryptCost)
	if err != nil {
		return "", err
	}
	rec.Set("secret_key_hash", string(skHash))
	if err := m.app.Save(rec); err != nil {
		return "", err
	}
	return sk, nil
}

// VerifySecret checks a bearer secret_key against the stored hash.
func (m *Manager) VerifySecret(appRec *core.Record, secret string) bool {
	hash := appRec.GetString("secret_key_hash")
	if hash == "" || secret == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

// --- Users ---

func (m *Manager) Signup(appRec *core.Record, email, password, name string, meta map[string]any) (User, Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !ValidEmail(email) {
		return User{}, Session{}, ErrInvalidEmail
	}
	if len(password) < 8 {
		return User{}, Session{}, ErrWeakPassword
	}

	dup, _ := m.app.FindFirstRecordByFilter(CollectionUsers,
		"app_id = {:a} && email = {:e}", map[string]any{"a": appRec.Id, "e": email})
	if dup != nil {
		return User{}, Session{}, ErrDuplicate
	}

	col, err := m.app.FindCollectionByNameOrId(CollectionUsers)
	if err != nil {
		return User{}, Session{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return User{}, Session{}, err
	}
	rec := core.NewRecord(col)
	rec.Set("app_id", appRec.Id)
	rec.Set("email", email)
	rec.Set("password_hash", string(hash))
	rec.Set("name", name)
	if meta != nil {
		rec.Set("metadata", meta)
	}
	if err := m.app.Save(rec); err != nil {
		return User{}, Session{}, err
	}
	u := m.userFromRecord(rec)
	sess, err := m.issueSession(appRec, rec)
	if err != nil {
		return u, Session{}, err
	}
	return u, sess, nil
}

func (m *Manager) Login(appRec *core.Record, email, password string) (Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	rec, err := m.app.FindFirstRecordByFilter(CollectionUsers,
		"app_id = {:a} && email = {:e}", map[string]any{"a": appRec.Id, "e": email})
	if err != nil || rec == nil {
		return Session{}, ErrBadPassword
	}
	if rec.GetBool("disabled") {
		return Session{}, ErrDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(rec.GetString("password_hash")), []byte(password)); err != nil {
		return Session{}, ErrBadPassword
	}
	return m.issueSession(appRec, rec)
}

func (m *Manager) ListUsers(appRec *core.Record, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 500
	}
	records, err := m.app.FindRecordsByFilter(CollectionUsers,
		"app_id = {:a}", "-created", limit, 0, map[string]any{"a": appRec.Id})
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(records))
	for _, r := range records {
		out = append(out, m.userFromRecord(r))
	}
	return out, nil
}

func (m *Manager) DeleteUser(appRec *core.Record, userID string) error {
	rec, err := m.app.FindRecordById(CollectionUsers, userID)
	if err != nil || rec == nil || rec.GetString("app_id") != appRec.Id {
		return ErrUserNotFound
	}
	return m.app.Delete(rec)
}

func (m *Manager) SetDisabled(appRec *core.Record, userID string, disabled bool) error {
	rec, err := m.app.FindRecordById(CollectionUsers, userID)
	if err != nil || rec == nil || rec.GetString("app_id") != appRec.Id {
		return ErrUserNotFound
	}
	rec.Set("disabled", disabled)
	return m.app.Save(rec)
}

// VerifyJWT validates a bearer JWT and returns (app, user).
func (m *Manager) VerifyJWT(token string) (*core.Record, *core.Record, error) {
	// Parse claims unverified first to get the app_id, then look up the secret.
	claims, err := security.ParseUnverifiedJWT(token)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid token: %w", err)
	}
	appID, _ := claims["app_id"].(string)
	userID, _ := claims["sub"].(string)
	if appID == "" || userID == "" {
		return nil, nil, errors.New("invalid token: missing app_id/sub")
	}
	appRec, err := m.app.FindRecordById(CollectionApps, appID)
	if err != nil || appRec == nil {
		return nil, nil, ErrAppNotFound
	}
	verified, err := security.ParseJWT(token, appRec.GetString("jwt_secret"))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid token: %w", err)
	}
	sub, _ := verified["sub"].(string)
	if sub != userID {
		return nil, nil, errors.New("invalid token: sub mismatch")
	}
	userRec, err := m.app.FindRecordById(CollectionUsers, userID)
	if err != nil || userRec == nil {
		return nil, nil, ErrUserNotFound
	}
	if userRec.GetString("app_id") != appID {
		return nil, nil, errors.New("invalid token: user/app mismatch")
	}
	if userRec.GetBool("disabled") {
		return nil, nil, ErrDisabled
	}
	return appRec, userRec, nil
}

// --- helpers ---

func (m *Manager) issueSession(appRec, userRec *core.Record) (Session, error) {
	ttl := appRec.GetInt("session_ttl_hours")
	if ttl <= 0 {
		ttl = defaultSessionTTLHours
	}
	duration := time.Duration(ttl) * time.Hour
	claims := jwt.MapClaims{
		"sub":    userRec.Id,
		"app_id": appRec.Id,
		"email":  userRec.GetString("email"),
	}
	token, err := security.NewJWT(claims, appRec.GetString("jwt_secret"), duration)
	if err != nil {
		return Session{}, err
	}
	return Session{
		Token:     token,
		ExpiresAt: time.Now().Add(duration).Unix(),
		User:      m.userFromRecord(userRec),
	}, nil
}

func (m *Manager) appFromRecord(rec *core.Record) App {
	return App{
		ID:              rec.Id,
		Name:            rec.GetString("name"),
		Slug:            rec.GetString("slug"),
		Description:     rec.GetString("description"),
		PublishableKey:  rec.GetString("publishable_key"),
		SessionTTLHours: rec.GetInt("session_ttl_hours"),
		Created:         rec.GetDateTime("created").String(),
	}
}

func (m *Manager) userFromRecord(rec *core.Record) User {
	u := User{
		ID:       rec.Id,
		AppID:    rec.GetString("app_id"),
		Email:    rec.GetString("email"),
		Name:     rec.GetString("name"),
		Disabled: rec.GetBool("disabled"),
		Created:  rec.GetDateTime("created").String(),
	}
	if meta, ok := rec.Get("metadata").(map[string]any); ok {
		u.Metadata = meta
	}
	return u
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
