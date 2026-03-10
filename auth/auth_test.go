package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kristofer/postdoc/auth"
)

func init() {
	auth.Init([]byte("test-secret-key-for-unit-tests"), false)
}

func TestGenerateAndValidateToken(t *testing.T) {
	token, err := auth.GenerateToken("alice")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	username, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if username != "alice" {
		t.Errorf("expected 'alice', got %q", username)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	_, err := auth.ValidateToken("not.a.valid.jwt")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestValidateToken_Tampered(t *testing.T) {
	token, _ := auth.GenerateToken("admin")
	tampered := token + "x"
	_, err := auth.ValidateToken(tampered)
	if err == nil {
		t.Error("expected error for tampered token")
	}
}

func TestMiddleware_RedirectsUnauthenticated(t *testing.T) {
	protected := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestMiddleware_AllowsAuthenticated(t *testing.T) {
	token, _ := auth.GenerateToken("admin")

	protected := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "postdoc_session", Value: token})
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestSetAndClearCookie(t *testing.T) {
	token, _ := auth.GenerateToken("bob")

	// SetCookie
	rr := httptest.NewRecorder()
	auth.SetCookie(rr, token)
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a cookie to be set")
	}
	if cookies[0].Value != token {
		t.Errorf("cookie value mismatch")
	}

	// ClearCookie
	rr2 := httptest.NewRecorder()
	auth.ClearCookie(rr2)
	cleared := rr2.Result().Cookies()
	if len(cleared) == 0 {
		t.Fatal("expected a clearing cookie")
	}
	if cleared[0].MaxAge != -1 {
		t.Errorf("expected MaxAge=-1 for clearing cookie, got %d", cleared[0].MaxAge)
	}
}

func TestUsernameFromRequest(t *testing.T) {
	token, _ := auth.GenerateToken("charlie")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "postdoc_session", Value: token})

	username, err := auth.UsernameFromRequest(req)
	if err != nil {
		t.Fatalf("username from request: %v", err)
	}
	if username != "charlie" {
		t.Errorf("expected 'charlie', got %q", username)
	}
}

func TestUsernameFromRequest_NoCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := auth.UsernameFromRequest(req)
	if err == nil {
		t.Error("expected error when no cookie")
	}
}
