package handlers_test

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kristofer/postdoc/auth"
	"github.com/kristofer/postdoc/db"
	"github.com/kristofer/postdoc/handlers"
)

func init() {
	auth.Init([]byte("handler-test-secret"), false)
}

func makeAuthTemplates(t *testing.T) *template.Template {
	t.Helper()
	const tmplSrc = `
{{define "login.html"}}LOGIN{{if .Error}} ERROR:{{.Error}}{{end}}{{end}}
{{define "admin.html"}}ADMIN ADMINS:{{range .Admins}}{{.Username}},{{end}} ERROR:{{.Error}}{{end}}
`
	tmpl, err := template.New("root").Parse(tmplSrc)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	return tmpl
}

func openAuthTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "auth_test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func sessionCookie(t *testing.T, username string) *http.Cookie {
	t.Helper()
	token, err := auth.GenerateToken(username)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return &http.Cookie{Name: "postdoc_session", Value: token}
}

// TestLoginHandler_GET checks the login form is served.
func TestLoginHandler_GET(t *testing.T) {
	database := openAuthTestDB(t)
	tmpl := makeAuthTemplates(t)

	h := handlers.LoginHandler(database, tmpl)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "LOGIN") {
		t.Error("expected login template output")
	}
}

// TestLoginHandler_POST_Success checks valid credentials issue a cookie and redirect.
func TestLoginHandler_POST_Success(t *testing.T) {
	database := openAuthTestDB(t)
	tmpl := makeAuthTemplates(t)

	h := handlers.LoginHandler(database, tmpl)

	form := url.Values{"username": {"admin"}, "password": {"foobar"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d: %s", rr.Code, rr.Body.String())
	}
	var hasCookie bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "postdoc_session" && c.Value != "" {
			hasCookie = true
		}
	}
	if !hasCookie {
		t.Error("expected session cookie to be set")
	}
}

// TestLoginHandler_POST_BadCredentials checks wrong creds re-show login with error.
func TestLoginHandler_POST_BadCredentials(t *testing.T) {
	database := openAuthTestDB(t)
	tmpl := makeAuthTemplates(t)

	h := handlers.LoginHandler(database, tmpl)

	form := url.Values{"username": {"admin"}, "password": {"wrongpassword"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-show form), got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid") {
		t.Errorf("expected error text in response, got: %s", rr.Body.String())
	}
}

// TestLogoutHandler checks the cookie is cleared and user is redirected to /login.
func TestLogoutHandler(t *testing.T) {
	h := handlers.LogoutHandler()
	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if rr.Header().Get("Location") != "/login" {
		t.Errorf("expected redirect to /login, got %q", rr.Header().Get("Location"))
	}
	var cleared bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "postdoc_session" && c.MaxAge == -1 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("expected session cookie to be cleared (MaxAge=-1)")
	}
}

// TestAdminHandler_ListsAdmins checks the admin page lists existing admins.
func TestAdminHandler_ListsAdmins(t *testing.T) {
	database := openAuthTestDB(t)
	tmpl := makeAuthTemplates(t)

	h := handlers.AdminHandler(database, tmpl)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "admin,") {
		t.Errorf("expected default admin listed, got: %s", rr.Body.String())
	}
}

// TestAddAdminHandler_Success creates a new admin and lists it.
func TestAddAdminHandler_Success(t *testing.T) {
	database := openAuthTestDB(t)
	tmpl := makeAuthTemplates(t)

	h := handlers.AddAdminHandler(database, tmpl)
	form := url.Values{"username": {"newguy"}, "password": {"pass123"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "newguy,") {
		t.Errorf("expected 'newguy' in admin list: %s", rr.Body.String())
	}
}

// TestAddAdminHandler_DuplicateUsername returns an error message.
func TestAddAdminHandler_DuplicateUsername(t *testing.T) {
	database := openAuthTestDB(t)
	tmpl := makeAuthTemplates(t)

	h := handlers.AddAdminHandler(database, tmpl)
	// Try to add "admin" again (already seeded).
	form := url.Values{"username": {"admin"}, "password": {"another"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Could not create") {
		t.Errorf("expected duplicate error text: %s", rr.Body.String())
	}
}

// TestDeleteAdminHandler_Success removes an admin.
func TestDeleteAdminHandler_Success(t *testing.T) {
	database := openAuthTestDB(t)
	tmpl := makeAuthTemplates(t)

	// Add a second admin to delete.
	if err := database.CreateAdmin("todelete", "pass"); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	target, err := database.GetAdminByUsername("todelete")
	if err != nil || target == nil {
		t.Fatalf("get admin: %v", err)
	}

	h := handlers.DeleteAdminHandler(database, tmpl)
	form := url.Values{"id": {fmt.Sprintf("%d", target.ID)}}
	req := httptest.NewRequest(http.MethodPost, "/admin/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "todelete,") {
		t.Error("deleted admin should not appear in list")
	}
}

// TestDeleteAdminHandler_CannotDeleteSelf prevents self-deletion.
func TestDeleteAdminHandler_CannotDeleteSelf(t *testing.T) {
	database := openAuthTestDB(t)
	tmpl := makeAuthTemplates(t)

	adminRec, _ := database.GetAdminByUsername("admin")

	h := handlers.DeleteAdminHandler(database, tmpl)
	form := url.Values{"id": {fmt.Sprintf("%d", adminRec.ID)}}
	req := httptest.NewRequest(http.MethodPost, "/admin/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	// Admin should still be listed.
	if !strings.Contains(rr.Body.String(), "admin,") {
		t.Errorf("admin should still be listed after self-delete attempt: %s", rr.Body.String())
	}
	// Error message expected.
	if !strings.Contains(rr.Body.String(), "cannot delete") {
		t.Errorf("expected self-delete error text: %s", rr.Body.String())
	}
}
