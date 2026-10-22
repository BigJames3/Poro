package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// An administrator suspends an account: its access token is refused at once
// (the registry every service reads), it can neither refresh nor sign in, and
// it signs in again once reinstated (ADR-0019).
func TestAdminSuspendsAndReinstatesAnAccount(t *testing.T) {
	api, _, _ := startAuthAPI(t)
	register := func(email string) map[string]any {
		status, body := api.call(http.MethodPost, "/api/v1/auth/email/register", "",
			`{"email":"`+email+`","password":"un-bon-mdp","full_name":"Test"}`)
		require.Equal(t, http.StatusCreated, status)
		return body
	}
	login := func(email string) (int, map[string]any) {
		return api.call(http.MethodPost, "/api/v1/auth/email/login", "", `{"email":"`+email+`","password":"un-bon-mdp"}`)
	}

	adminBody := register("admin@poro.test")
	member := register("membre@poro.test")
	memberID := member["user"].(map[string]any)["id"].(string)
	suspend := "/api/v1/auth/admin/users/" + memberID + "/suspend"
	reinstate := "/api/v1/auth/admin/users/" + memberID + "/reinstate"

	status, body := api.call(http.MethodPost, suspend, adminBody["access_token"].(string), "")
	require.Equal(t, http.StatusForbidden, status, "an account without a staff role cannot suspend")
	require.Equal(t, "forbidden", body["code"])

	adminID := adminBody["user"].(map[string]any)["id"].(string)
	_, err := api.pool.Exec(context.Background(),
		`INSERT INTO user_roles (user_id, role, granted_at) VALUES ($1, 'ADMIN', now())`, adminID)
	require.NoError(t, err)
	status, adminBody = login("admin@poro.test")
	require.Equal(t, http.StatusOK, status)
	admin := adminBody["access_token"].(string)

	status, body = api.call(http.MethodPost, "/api/v1/auth/admin/users/"+adminID+"/suspend", admin, "")
	require.Equal(t, http.StatusUnprocessableEntity, status)
	require.Equal(t, "self_suspension", body["code"])

	// A moderator suspends members, never staff.
	moderatorBody := register("moderateur@poro.test")
	_, err = api.pool.Exec(context.Background(), `INSERT INTO user_roles (user_id, role, granted_at) VALUES ($1, 'MODERATOR', now())`,
		moderatorBody["user"].(map[string]any)["id"].(string))
	require.NoError(t, err)
	status, moderatorBody = login("moderateur@poro.test")
	require.Equal(t, http.StatusOK, status)
	status, body = api.call(http.MethodPost, "/api/v1/auth/admin/users/"+adminID+"/suspend", moderatorBody["access_token"].(string), "")
	require.Equal(t, http.StatusForbidden, status, "a moderator cannot suspend an admin")
	require.Equal(t, "forbidden", body["code"])

	status, body = api.call(http.MethodPost, suspend, admin, "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "SUSPENDED", body["status"])

	status, _ = api.call(http.MethodGet, "/api/v1/auth/me", member["access_token"].(string), "")
	require.Equal(t, http.StatusUnauthorized, status, "the access token is refused at once")
	status, _ = api.call(http.MethodPost, "/api/v1/auth/refresh", "", `{"refresh_token":"`+member["refresh_token"].(string)+`"}`)
	require.Equal(t, http.StatusUnauthorized, status, "the refresh token was revoked")
	status, body = login("membre@poro.test")
	require.Equal(t, http.StatusForbidden, status)
	require.Equal(t, "account_blocked", body["code"])

	status, body = api.call(http.MethodPost, reinstate, admin, "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "PENDING", body["status"])
	status, body = login("membre@poro.test")
	require.Equal(t, http.StatusOK, status, "a reinstated account signs in again")
	require.Equal(t, "ACTIVE", body["user"].(map[string]any)["status"])
}
