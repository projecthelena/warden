package api

import "net/http"

// Role constants
const (
	RoleAdmin        = "admin"
	RoleEditor       = "editor"
	RoleViewer       = "viewer"
	RoleStatusViewer = "status_viewer"
)

const contextKeyUserRole contextKey = "userRole"

// roleLevel returns a numeric level for role hierarchy comparison.
// Higher level = more permissions.
func roleLevel(role string) int {
	switch role {
	case RoleAdmin:
		return 4
	case RoleEditor:
		return 3
	case RoleViewer:
		return 2
	case RoleStatusViewer:
		return 1
	default:
		return 0
	}
}

// hasPermission checks if userRole meets or exceeds the minimum required role.
func hasPermission(userRole, minimumRole string) bool {
	return roleLevel(userRole) >= roleLevel(minimumRole)
}

// getUserRole extracts the user's role from the request context.
func getUserRole(r *http.Request) string {
	role, ok := r.Context().Value(contextKeyUserRole).(string)
	if !ok {
		return ""
	}
	return role
}

// requireRole checks if the user has the minimum required role.
// Returns true if the user has permission, false if blocked (response already written).
func requireRole(w http.ResponseWriter, r *http.Request, minimumRole string) bool {
	role := getUserRole(r)
	if !hasPermission(role, minimumRole) {
		writeError(w, http.StatusForbidden, "your role ("+role+") does not have permission for this action")
		return false
	}
	return true
}

// RequireViewerMiddleware blocks status_viewer users from dashboard endpoints.
// status_viewer users can only access /api/auth/me and their assigned status pages.
func RequireViewerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := getUserRole(r)
		if role == RoleStatusViewer {
			writeError(w, http.StatusForbidden, "your role ("+role+") does not have permission for this action")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ValidRole returns true if the given role string is a valid RBAC role.
func ValidRole(role string) bool {
	switch role {
	case RoleAdmin, RoleEditor, RoleViewer, RoleStatusViewer:
		return true
	default:
		return false
	}
}
