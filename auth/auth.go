package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const cookieName = "postdoc_session"

// tokenTTL is how long a session token remains valid.
const tokenTTL = 24 * time.Hour

var errInvalidToken = errors.New("invalid or expired token")

var secret []byte
var secureCookie bool

// Init sets the HMAC secret used to sign JWT tokens and whether to require
// HTTPS for the session cookie. Must be called before any other function in
// this package.
func Init(s []byte, secure bool) {
	secret = s
	secureCookie = secure
}

// GenerateToken creates a signed JWT for the given username.
func GenerateToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(tokenTTL).Unix(),
		"iat": time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(secret)
}

// ValidateToken parses and validates a JWT string and returns the username.
func ValidateToken(tokenStr string) (string, error) {
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errInvalidToken
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !tok.Valid {
		return "", errInvalidToken
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return "", errInvalidToken
	}
	username, ok := claims["sub"].(string)
	if !ok || username == "" {
		return "", errInvalidToken
	}
	return username, nil
}

// SetCookie writes the JWT as an HTTP-only cookie on the response.
func SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(tokenTTL.Seconds()),
	})
}

// ClearCookie removes the session cookie.
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		MaxAge:   -1,
	})
}

// UsernameFromRequest extracts the authenticated username from the request's
// session cookie. Returns ("", error) when not authenticated.
func UsernameFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return "", errInvalidToken
	}
	return ValidateToken(cookie.Value)
}

// Middleware returns an HTTP middleware that requires a valid session cookie.
// Unauthenticated requests are redirected to /login.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := UsernameFromRequest(r); err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
