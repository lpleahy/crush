package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newDeviceTestClient(t *testing.T, userCodeHandler, tokenHandler, exchangeHandler http.HandlerFunc) *Client {
	t.Helper()
	mux := http.NewServeMux()
	if userCodeHandler != nil {
		mux.HandleFunc("/deviceauth/usercode", userCodeHandler)
	}
	if tokenHandler != nil {
		mux.HandleFunc("/deviceauth/token", tokenHandler)
	}
	if exchangeHandler != nil {
		mux.HandleFunc("/oauth/token", exchangeHandler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient()
	c.DeviceUserCodeURL = srv.URL + "/deviceauth/usercode"
	c.DeviceTokenURL = srv.URL + "/deviceauth/token"
	c.TokenURL = srv.URL + "/oauth/token"
	return c
}

func TestRequestDeviceCode_Success(t *testing.T) {
	c := newDeviceTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req["client_id"] != DefaultClientID {
				t.Errorf("client_id = %q", req["client_id"])
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"device_auth_id":"dev-1","user_code":"ABC-DEF","interval":3}`))
		}, nil, nil)

	dc, err := c.RequestDeviceCode(context.Background())
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if dc.DeviceAuthID != "dev-1" {
		t.Errorf("DeviceAuthID = %q", dc.DeviceAuthID)
	}
	if dc.UserCode != "ABC-DEF" {
		t.Errorf("UserCode = %q", dc.UserCode)
	}
	if dc.Interval != 3*time.Second {
		t.Errorf("Interval = %s, want 3s", dc.Interval)
	}
	if dc.VerificationURL != DefaultDeviceVerificationURL {
		t.Errorf("VerificationURL = %q", dc.VerificationURL)
	}
}

func TestRequestDeviceCode_IntervalAsString(t *testing.T) {
	c := newDeviceTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"device_auth_id":"d","user_code":"X","interval":"7"}`))
		}, nil, nil)

	dc, err := c.RequestDeviceCode(context.Background())
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if dc.Interval != 7*time.Second {
		t.Errorf("Interval = %s, want 7s", dc.Interval)
	}
}

func TestRequestDeviceCode_UsercodeFieldAlias(t *testing.T) {
	c := newDeviceTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"device_auth_id":"d","usercode":"FROM-ALT","interval":1}`))
		}, nil, nil)

	dc, err := c.RequestDeviceCode(context.Background())
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if dc.UserCode != "FROM-ALT" {
		t.Errorf("UserCode = %q, want FROM-ALT", dc.UserCode)
	}
}

func TestRequestDeviceCode_404IsFriendly(t *testing.T) {
	c := newDeviceTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}, nil, nil)

	_, err := c.RequestDeviceCode(context.Background())
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "device code login is not enabled") {
		t.Errorf("want friendly disabled message, got %q", err.Error())
	}
}

func TestRequestDeviceCode_MissingFields(t *testing.T) {
	c := newDeviceTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"interval":5}`))
		}, nil, nil)

	if _, err := c.RequestDeviceCode(context.Background()); err == nil {
		t.Fatal("expected error when device_auth_id + user_code missing")
	}
}

func TestPollForDeviceToken_HappyPath(t *testing.T) {
	var pollCount atomic.Int32
	c := newDeviceTestClient(t,
		nil,
		func(w http.ResponseWriter, r *http.Request) {
			n := pollCount.Add(1)
			if n < 3 {
				// Server still waiting on user — return 403 the first two times.
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{"authorization_code":"ac_xyz","code_challenge":"chal","code_verifier":"verif"}`))
		},
		func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse form: %v", err)
			}
			if r.Form.Get("code") != "ac_xyz" {
				t.Errorf("code = %q", r.Form.Get("code"))
			}
			if r.Form.Get("code_verifier") != "verif" {
				t.Errorf("code_verifier = %q", r.Form.Get("code_verifier"))
			}
			if r.Form.Get("redirect_uri") != DefaultDeviceRedirectURI {
				t.Errorf("redirect_uri = %q, want %q", r.Form.Get("redirect_uri"), DefaultDeviceRedirectURI)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"at-final","refresh_token":"rt-final","expires_in":300}`))
		})

	dc := &DeviceCode{
		DeviceAuthID:    "dev-1",
		UserCode:        "ABC-DEF",
		VerificationURL: DefaultDeviceVerificationURL,
		Interval:        20 * time.Millisecond,
	}
	tok, err := c.PollForDeviceToken(context.Background(), dc)
	if err != nil {
		t.Fatalf("PollForDeviceToken: %v", err)
	}
	if tok.AccessToken != "at-final" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
	if tok.RefreshToken != "rt-final" {
		t.Errorf("RefreshToken = %q", tok.RefreshToken)
	}
	if pollCount.Load() != 3 {
		t.Errorf("expected 3 polls before success, got %d", pollCount.Load())
	}
}

func TestPollForDeviceToken_HardError(t *testing.T) {
	c := newDeviceTestClient(t, nil,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		}, nil)

	dc := &DeviceCode{
		DeviceAuthID: "d",
		UserCode:     "X",
		Interval:     5 * time.Millisecond,
	}
	_, err := c.PollForDeviceToken(context.Background(), dc)
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should include 500 + body, got %q", err.Error())
	}
}

func TestPollForDeviceToken_CtxCancel(t *testing.T) {
	c := newDeviceTestClient(t, nil,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden) // keep polling forever
		}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(40 * time.Millisecond)
		cancel()
	}()

	dc := &DeviceCode{DeviceAuthID: "d", UserCode: "X", Interval: 10 * time.Millisecond}
	if _, err := c.PollForDeviceToken(ctx, dc); err == nil {
		t.Fatal("expected ctx error, got nil")
	}
}

func TestPollForDeviceToken_NilCode(t *testing.T) {
	c := NewClient()
	if _, err := c.PollForDeviceToken(context.Background(), nil); err == nil {
		t.Fatal("expected error on nil device code")
	}
}
