package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/kristofer/postdoc/auth"
	"github.com/kristofer/postdoc/db"
)

// LoginHandler serves GET /login (form) and processes POST /login (credentials).
func LoginHandler(database *db.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if err := tmpl.ExecuteTemplate(w, "login.html", nil); err != nil {
				log.Printf("template error: %v", err)
			}
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		admin, err := database.AuthenticateAdmin(username, password)
		if err != nil {
			log.Printf("auth error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if admin == nil {
			if err := tmpl.ExecuteTemplate(w, "login.html", map[string]string{
				"Error": "Invalid username or password.",
			}); err != nil {
				log.Printf("template error: %v", err)
			}
			return
		}

		token, err := auth.GenerateToken(admin.Username)
		if err != nil {
			log.Printf("token generation error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		auth.SetCookie(w, token)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// LogoutHandler clears the session cookie and redirects to /login.
func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth.ClearCookie(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// AdminHandler serves the admin management page (GET) and handles admin actions (POST).
func AdminHandler(database *db.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)

		admins, err := database.ListAdmins()
		if err != nil {
			log.Printf("list admins error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Admins":   admins,
			"Username": username,
		}
		if err := tmpl.ExecuteTemplate(w, "admin.html", data); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}

// AddAdminHandler handles POST /admin/add — creates a new admin account.
func AddAdminHandler(database *db.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		newUsername := strings.TrimSpace(r.FormValue("username"))
		newPassword := r.FormValue("password")

		var flashErr string
		if newUsername == "" || newPassword == "" {
			flashErr = "Username and password are required."
		} else if err := database.CreateAdmin(newUsername, newPassword); err != nil {
			flashErr = "Could not create admin (username may already exist)."
			log.Printf("create admin error: %v", err)
		}

		currentUser, _ := auth.UsernameFromRequest(r)
		admins, err := database.ListAdmins()
		if err != nil {
			log.Printf("list admins error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Admins":   admins,
			"Username": currentUser,
			"Error":    flashErr,
		}
		if err := tmpl.ExecuteTemplate(w, "admin.html", data); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}

// ChangePasswordHandler handles POST /admin/change-password — updates the
// current user's password after verifying the current password.
func ChangePasswordHandler(database *db.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		currentPassword := r.FormValue("current_password")
		newPassword := r.FormValue("new_password")
		confirmPassword := r.FormValue("confirm_password")

		currentUser, _ := auth.UsernameFromRequest(r)

		var passwordErr, passwordSuccess string
		if newPassword == "" {
			passwordErr = "New password is required."
		} else if newPassword != confirmPassword {
			passwordErr = "New passwords do not match."
		} else {
			admin, err := database.AuthenticateAdmin(currentUser, currentPassword)
			if err != nil {
				log.Printf("change password auth error: %v", err)
				passwordErr = "Internal server error."
			} else if admin == nil {
				passwordErr = "Current password is incorrect."
			} else if err := database.ChangePassword(currentUser, newPassword); err != nil {
				log.Printf("change password error: %v", err)
				passwordErr = "Could not update password."
			} else {
				passwordSuccess = "Password updated successfully."
			}
		}

		admins, err := database.ListAdmins()
		if err != nil {
			log.Printf("list admins error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Admins":          admins,
			"Username":        currentUser,
			"PasswordError":   passwordErr,
			"PasswordSuccess": passwordSuccess,
		}
		if err := tmpl.ExecuteTemplate(w, "admin.html", data); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}

func DeleteAdminHandler(database *db.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		idStr := r.FormValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		// Prevent deleting yourself.
		currentUser, _ := auth.UsernameFromRequest(r)
		adminToDelete, err := database.GetAdminByID(id)
		if err != nil {
			log.Printf("get admin error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		var flashErr string
		if adminToDelete != nil && adminToDelete.Username == currentUser {
			flashErr = "You cannot delete your own account."
		} else if adminToDelete != nil {
			if err := database.DeleteAdmin(id); err != nil {
				log.Printf("delete admin error: %v", err)
				flashErr = "Could not delete admin."
			}
		}

		admins, err := database.ListAdmins()
		if err != nil {
			log.Printf("list admins error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Admins":   admins,
			"Username": currentUser,
			"Error":    flashErr,
		}
		if err := tmpl.ExecuteTemplate(w, "admin.html", data); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}
