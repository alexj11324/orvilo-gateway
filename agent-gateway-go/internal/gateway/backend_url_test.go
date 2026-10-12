package gateway

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestConfiguredAPIKeyServerURL(t *testing.T) {
	for _, test := range []struct {
		name       string
		configured string
		requested  string
		valid      bool
	}{
		{"configured URL", "https://orvilo.example", "https://orvilo.example", true},
		{"trailing slash", "https://orvilo.example/", "https://orvilo.example", true},
		{"configured local backend", "http://localhost:3010", "http://localhost:3010/", true},
		{"different URL", "https://orvilo.example", "https://different.example", false},
		{"different scheme", "https://orvilo.example", "http://orvilo.example", false},
		{"different port", "https://orvilo.example", "https://orvilo.example:8443", false},
		{"different path", "https://orvilo.example", "https://orvilo.example/other", false},
		{"missing configured URL", "", "https://orvilo.example", false},
		{"missing requested URL", "https://orvilo.example", "", false},
		{"invalid configured scheme", "file:///backend", "file:///backend", false},
		{"configured user info", "https://name@orvilo.example", "https://name@orvilo.example", false},
		{"configured query", "https://orvilo.example?key=value", "https://orvilo.example?key=value", false},
		{"configured fragment", "https://orvilo.example#section", "https://orvilo.example#section", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateAPIKeyServerURL(test.configured, test.requested); (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}

type backendURLTransport func(*http.Request) (*http.Response, error)

func (f backendURLTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAPIKeyAuthUsesConfiguredBackend(t *testing.T) {
	previousClient := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previousClient })
	calls := 0
	http.DefaultClient = &http.Client{Transport: backendURLTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != "https://orvilo.example/api/v1/users/me?includeCount=0" {
			t.Fatalf("unexpected configured backend request: %s", req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":{"id":"fixture-user"}}`)),
		}, nil
	})}
	resolver := newAuthResolver(Config{OrviloAPIBaseURL: "https://orvilo.example"})
	userID, err := resolver.resolve(context.Background(), "", authMessage{
		ServerURL: "https://orvilo.example/",
		Token:     "unit-test-api-key",
		TokenType: "apiKey",
	})
	if err != nil || userID != "fixture-user" || calls != 1 {
		t.Fatalf("configured backend auth: userID=%q calls=%d error=%v", userID, calls, err)
	}
	_, err = resolver.resolve(context.Background(), "", authMessage{
		ServerURL: "https://different.example",
		Token:     "unit-test-api-key",
		TokenType: "apiKey",
	})
	if err == nil || calls != 1 {
		t.Fatalf("different backend must be rejected before HTTP: calls=%d error=%v", calls, err)
	}
}

func TestConfigRequiresBackendURL(t *testing.T) {
	if err := (Config{ServiceToken: "unit-test-service-token"}).Validate(); err == nil {
		t.Fatal("missing backend URL must fail configuration validation")
	}
	if err := (Config{ServiceToken: "unit-test-service-token", OrviloAPIBaseURL: "https://orvilo.example"}).Validate(); err != nil {
		t.Fatalf("configured backend URL must pass: %v", err)
	}
}

func TestConfigFromEnvRequiresExplicitBackendURL(t *testing.T) {
	t.Setenv("SERVICE_TOKEN", "unit-test-service-token")
	t.Setenv("ORVILO_API_BASE_URL", "")
	t.Setenv("LOBE_API_BASE_URL", "")
	if err := ConfigFromEnv().Validate(); err == nil {
		t.Fatal("missing backend environment setting must fail validation")
	}
	t.Setenv("ORVILO_API_BASE_URL", "https://orvilo.example")
	if err := ConfigFromEnv().Validate(); err != nil {
		t.Fatalf("configured backend environment setting must pass: %v", err)
	}
}

func TestConfigFromEnvStillAcceptsTheUpstreamBackendName(t *testing.T) {
	t.Setenv("ORVILO_API_BASE_URL", "")
	t.Setenv("LOBE_API_BASE_URL", "https://legacy.example")
	if got := ConfigFromEnv().OrviloAPIBaseURL; got != "https://legacy.example" {
		t.Fatalf("backend URL = %q, want the value of LOBE_API_BASE_URL", got)
	}
	t.Setenv("ORVILO_API_BASE_URL", "https://orvilo.example")
	if got := ConfigFromEnv().OrviloAPIBaseURL; got != "https://orvilo.example" {
		t.Fatalf("backend URL = %q, want ORVILO_API_BASE_URL to win", got)
	}
}
