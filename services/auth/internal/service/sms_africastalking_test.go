package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/poro/auth/internal/config"
)

const atAPIKey = "atsk_test_key"

func TestAfricasTalkingSendsTheCode(t *testing.T) {
	var got *http.Request
	var form map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		got = r
		form = map[string]string{}
		for k := range r.PostForm {
			form[k] = r.PostForm.Get(k)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"SMSMessageData":{"Message":"Sent to 1/1 Total Cost: XOF 20","Recipients":[{"statusCode":101,"number":"+2250707070707","status":"Success","cost":"XOF 20","messageId":"ATXid_1"}]}}`))
	}))
	defer srv.Close()

	core, logs := observer.New(zap.InfoLevel)
	sender := testATSender(srv, "PORO", zap.New(core))
	require.NoError(t, sender.SendOTP(context.Background(), testPhone, "482913", 5*time.Minute))

	require.Equal(t, http.MethodPost, got.Method)
	require.Equal(t, atAPIKey, got.Header.Get("apiKey"))
	require.Equal(t, "application/json", got.Header.Get("Accept"))
	require.Equal(t, "application/x-www-form-urlencoded", got.Header.Get("Content-Type"))
	require.Equal(t, "poro", form["username"])
	require.Equal(t, testPhone, form["to"])
	require.Equal(t, "PORO", form["from"])
	require.Equal(t, "1", form["bulkSMSMode"])
	require.Equal(t, "Votre code PORO : 482913. Valable 5 min. Ne le partagez jamais.", form["message"])

	require.Equal(t, 1, logs.Len())
	fields := logs.All()[0].ContextMap()
	require.Equal(t, "ATXid_1", fields["message_id"])
	require.Equal(t, "+225****07", fields["phone"])
	require.NotContains(t, fields, "code")
}

func TestAfricasTalkingOmitsEmptySenderID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		_, hasFrom := r.PostForm["from"]
		require.False(t, hasFrom, "an empty sender ID uses the account default")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"SMSMessageData":{"Recipients":[{"statusCode":102,"status":"Queued"}]}}`))
	}))
	defer srv.Close()
	require.NoError(t, testATSender(srv, "", zap.NewNop()).SendOTP(context.Background(), testPhone, "000001", time.Minute))
}

func TestAfricasTalkingFailures(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{name: "invalid number", status: 201, body: `{"SMSMessageData":{"Recipients":[{"statusCode":403,"status":"InvalidPhoneNumber"}]}}`, want: ErrPhoneUnreachable},
		{name: "unsupported number", status: 201, body: `{"SMSMessageData":{"Recipients":[{"statusCode":404,"status":"UnsupportedNumberType"}]}}`, want: ErrPhoneUnreachable},
		{name: "blacklisted user", status: 201, body: `{"SMSMessageData":{"Recipients":[{"statusCode":406,"status":"UserInBlacklist"}]}}`, want: ErrPhoneUnreachable},
		{name: "insufficient balance", status: 201, body: `{"SMSMessageData":{"Recipients":[{"statusCode":405,"status":"InsufficientBalance"}]}}`, want: ErrSMSUnavailable},
		{name: "invalid sender id", status: 201, body: `{"SMSMessageData":{"Recipients":[{"statusCode":402,"status":"InvalidSenderId"}]}}`, want: ErrSMSUnavailable},
		{name: "no recipient", status: 201, body: `{"SMSMessageData":{"Message":"InvalidSenderId","Recipients":[]}}`, want: ErrSMSUnavailable},
		{name: "bad api key", status: 401, body: `The supplied authentication is invalid`, want: ErrSMSUnavailable},
		{name: "server error", status: 500, body: ``, want: ErrSMSUnavailable},
		{name: "not json", status: 201, body: `<html>`, want: ErrSMSUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			err := testATSender(srv, "", zap.NewNop()).SendOTP(context.Background(), testPhone, "123456", time.Minute)
			require.ErrorIs(t, err, tc.want)
			require.NotContains(t, err.Error(), atAPIKey)
			require.NotContains(t, err.Error(), "123456", "errors never carry the code")
		})
	}
}

func TestAfricasTalkingNetworkErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	sender := testATSender(srv, "", zap.NewNop())
	sender.client.Timeout = 20 * time.Millisecond
	err := sender.SendOTP(context.Background(), testPhone, "123456", time.Minute)
	require.ErrorIs(t, err, ErrSMSUnavailable)

	srv.Close()
	err = sender.SendOTP(context.Background(), testPhone, "123456", time.Minute)
	require.ErrorIs(t, err, ErrSMSUnavailable)
}

func TestAfricasTalkingEndpointSelection(t *testing.T) {
	live := newAfricasTalkingSender(&config.Config{AfricasTalkingUsername: "poro", AfricasTalkingAPIKey: "k"}, http.DefaultClient, zap.NewNop())
	require.Equal(t, africasTalkingLiveURL, live.endpoint)
	sandbox := newAfricasTalkingSender(&config.Config{AfricasTalkingUsername: "sandbox", AfricasTalkingAPIKey: "k"}, http.DefaultClient, zap.NewNop())
	require.Equal(t, africasTalkingSandboxURL, sandbox.endpoint)
}

func TestOTPMessageLanguage(t *testing.T) {
	require.Equal(t, "Your PORO code is 123456. It expires in 5 min. Never share it.", otpMessage("+2348012345678", "123456", 5*time.Minute))
	for _, phone := range []string{"+2250707070707", "+221771234567", "+237650000000"} {
		require.True(t, strings.HasPrefix(otpMessage(phone, "123456", 90*time.Second), "Votre code PORO : 123456. Valable 2 min."), phone)
	}
}

func TestNewSMSSenderAfricasTalking(t *testing.T) {
	sender, err := NewSMSSender(&config.Config{AppEnv: config.EnvProd, SMSProvider: config.SMSProviderAfricasTalking, AfricasTalkingUsername: "poro", AfricasTalkingAPIKey: "k"}, zap.NewNop())
	require.NoError(t, err)
	require.IsType(t, &africasTalkingSender{}, sender)

	_, err = NewSMSSender(&config.Config{AppEnv: config.EnvProd, SMSProvider: config.SMSProviderAfricasTalking, AfricasTalkingUsername: "poro"}, zap.NewNop())
	require.Error(t, err)
}

func testATSender(srv *httptest.Server, senderID string, log *zap.Logger) *africasTalkingSender {
	s := newAfricasTalkingSender(&config.Config{
		AfricasTalkingUsername: "poro",
		AfricasTalkingAPIKey:   atAPIKey,
		AfricasTalkingSenderID: senderID,
	}, &http.Client{Timeout: time.Second}, log)
	s.endpoint = srv.URL
	return s
}
