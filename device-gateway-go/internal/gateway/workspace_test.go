package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWorkspaceJWTBoundToPool(t *testing.T) {
	jwks, sign := testJWTClaimsSigner(t)
	server := httptest.NewServer(NewServer(Config{ServiceToken: "service-token", JWKSPublicKey: jwks}).Routes())
	defer server.Close()
	claims := func(sub, workspace, purpose string) map[string]any {
		return map[string]any{"sub": sub, "workspace_id": workspace, "purpose": purpose, "exp": time.Now().Add(time.Minute).Unix()}
	}
	for _, tc := range []struct {
		name, path string
		claims     map[string]any
		accepted   bool
	}{
		{"workspace", "/ws?workspaceId=w1&deviceId=shared", claims("w1", "w1", "workspace-device-connect"), true},
		{"wrong-sub", "/ws?workspaceId=w1", claims("w2", "w1", "workspace-device-connect"), false},
		{"wrong-workspace", "/ws?workspaceId=w1", claims("w1", "w2", "workspace-device-connect"), false},
		{"wrong-purpose", "/ws?workspaceId=w1", claims("w1", "w1", "cli-sandbox"), false},
		{"personal-rejects-workspace", "/ws?userId=w1", claims("w1", "w1", "workspace-device-connect"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := dialTestWS(t, server.URL, tc.path)
			defer ws.conn.Close()
			ws.sendJSON(t, map[string]any{"type": "auth", "token": sign(tc.claims), "tokenType": "jwt"})
			result := ws.readJSON(t)
			expected := "auth_failed"
			if tc.accepted {
				expected = "auth_success"
			}
			if result["type"] != expected {
				t.Fatalf("expected %s, got %#v", expected, result)
			}
		})
	}
}

func TestAPIKeyRejectsUnconfiguredServer(t *testing.T) {
	calls := 0
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"data":{"id":"victim"},"success":true}`)
	}))
	defer fake.Close()
	server := httptest.NewServer(NewServer(Config{ServiceToken: "service-token"}).Routes())
	defer server.Close()
	ws := dialTestWS(t, server.URL, "/ws?userId=victim")
	defer ws.conn.Close()
	ws.sendJSON(t, map[string]any{"type": "auth", "tokenType": "apiKey", "token": "fake-key", "serverUrl": fake.URL})
	if result := ws.readJSON(t); result["type"] != "auth_failed" {
		t.Fatalf("untrusted server authenticated user: %#v", result)
	}
	if calls != 0 {
		t.Fatal("untrusted server received validation request")
	}
}

func TestWorkspacePoolsAndPermissionRPC(t *testing.T) {
	jwks, sign := testJWTClaimsSigner(t)
	server := httptest.NewServer(NewServer(Config{ServiceToken: "service-token", JWKSPublicKey: jwks}).Routes())
	defer server.Close()
	personal := dialTestWS(t, server.URL, "/ws?userId=member-a&deviceId=same-device&connectionId=personal")
	defer personal.conn.Close()
	personal.sendJSON(t, map[string]any{"type": "auth", "token": "service-token"})
	if result := personal.readJSON(t); result["type"] != "auth_success" {
		t.Fatal(result)
	}
	workspace := dialTestWS(t, server.URL, "/ws?workspaceId=shared&deviceId=same-device&connectionId=workspace")
	defer workspace.conn.Close()
	workspace.sendJSON(t, map[string]any{"type": "auth", "token": sign(map[string]any{"sub": "shared", "workspace_id": "shared", "purpose": "workspace-device-connect", "exp": time.Now().Add(time.Minute).Unix()})})
	if result := workspace.readJSON(t); result["type"] != "auth_success" {
		t.Fatal(result)
	}
	for _, body := range []string{`{"userId":"member-a"}`, `{"userId":"member-a","workspaceId":"shared"}`, `{"userId":"member-b","workspaceId":"shared"}`} {
		res := postJSON(t, server.URL+"/api/device/status", "service-token", body)
		assertJSON(t, res, map[string]any{"online": true, "deviceCount": float64(1)})
	}
	assertJSON(t, postJSON(t, server.URL+"/api/device/status", "service-token", `{"userId":"member-b"}`), map[string]any{"online": false, "deviceCount": float64(0)})
	assertJSON(t, postJSON(t, server.URL+"/api/device/status", "service-token", `{"userId":"member-a","workspaceId":"other"}`), map[string]any{"online": false, "deviceCount": float64(0)})
	assertJSON(t, postJSON(t, server.URL+"/api/device/status", "service-token", `{"userId":"workspace:shared"}`), map[string]any{"online": false, "deviceCount": float64(0)})
	done := make(chan *http.Response, 1)
	go func() {
		done <- postJSON(t, server.URL+"/api/device/rpc", "service-token", `{"userId":"member-b","workspaceId":"shared","deviceId":"same-device","method":"listHeterogeneousAgentPermissions","params":{"type":"claude-code","configId":"mode","value":"manual"}}`)
	}()
	request := workspace.readJSON(t)
	if request["type"] != "rpc_request" || request["method"] != "listHeterogeneousAgentPermissions" {
		t.Fatal(request)
	}
	params := request["params"].(map[string]any)
	if params["configId"] != "mode" || params["value"] != "manual" {
		t.Fatal(params)
	}
	workspace.sendJSON(t, map[string]any{"type": "rpc_response", "requestId": request["requestId"], "result": map[string]any{"success": true, "data": "workspace-permissions"}})
	assertJSON(t, <-done, map[string]any{"success": true, "data": "workspace-permissions"})
}

func TestAPIKeyConfiguredServerOnly(t *testing.T) {
	calls := 0
	trusted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users/me" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"data":{"id":"user"},"success":true}`)
	}))
	defer trusted.Close()
	malicious := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"data":{"id":"user"},"success":true}`)
	}))
	defer malicious.Close()
	server := httptest.NewServer(NewServer(Config{ServiceToken: "service-token", OrviloAPIBaseURL: trusted.URL}).Routes())
	defer server.Close()
	for _, tc := range []struct {
		url      string
		expected string
	}{{malicious.URL, "auth_failed"}, {trusted.URL + "/", "auth_success"}} {
		ws := dialTestWS(t, server.URL, "/ws?userId=user&connectionId="+tc.expected)
		ws.sendJSON(t, map[string]any{"type": "auth", "tokenType": "apiKey", "token": "key", "serverUrl": tc.url})
		if result := ws.readJSON(t); result["type"] != tc.expected {
			t.Fatal(result)
		}
		ws.conn.Close()
	}
	if calls != 0 {
		t.Fatal("malicious server received credential request")
	}
}
