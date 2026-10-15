package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestPasswordGuessingIsLockedPerAccount drives the sign-in lock through the
// HTTP API: failures from many IPs lock the account, not the IPs, and the
// lock answers 429 with Retry-After without checking the password.
func TestPasswordGuessingIsLockedPerAccount(t *testing.T) {
	api, _, mr := startAuthAPI(t)
	for _, email := range []string{"awa@poro.test", "kofi@poro.test"} {
		status, _ := api.call(http.MethodPost, "/api/v1/auth/email/register", "",
			`{"email":"`+email+`","password":"le-bon-mdp","full_name":"Test"}`)
		require.Equal(t, http.StatusCreated, status)
	}

	for i := range 5 {
		api.ip = fmt.Sprintf("41.202.%d.10", i) // a new attacker IP each time
		status, body := api.call(http.MethodPost, "/api/v1/auth/email/login", "", `{"email":"awa@poro.test","password":"devine-`+fmt.Sprint(i)+`"}`)
		require.Equal(t, http.StatusUnauthorized, status, i)
		require.Equal(t, "invalid_credentials", body["code"])
	}

	api.ip = "41.202.99.10"
	status, header, raw := api.send(http.MethodPost, "/api/v1/auth/email/login", "", `{"email":"awa@poro.test","password":"le-bon-mdp"}`)
	require.Equal(t, http.StatusTooManyRequests, status, "locked even with the right password")
	require.Contains(t, string(raw), `"login_throttled"`)
	require.Equal(t, "60", header.Get("Retry-After"))

	status, _ = api.call(http.MethodPost, "/api/v1/auth/email/login", "", `{"email":"kofi@poro.test","password":"le-bon-mdp"}`)
	require.Equal(t, http.StatusOK, status, "other accounts are not affected")

	mr.FastForward(time.Minute)
	status, _ = api.call(http.MethodPost, "/api/v1/auth/email/login", "", `{"email":"awa@poro.test","password":"le-bon-mdp"}`)
	require.Equal(t, http.StatusOK, status, "the lock lifts on time")
}
