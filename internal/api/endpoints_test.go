package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
)

func newTestLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.DebugLevel)
	return l
}

func TestFetchConfig_Success(t *testing.T) {
	cfg := NodeConfig{
		ServerPort:    443,
		ServerName:    "test.example.com",
		PaddingScheme: []string{"stop=8", "0=30-30"},
	}
	cfg.BaseConfig.PushInterval = 60
	cfg.BaseConfig.PullInterval = 60

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/server/UniProxy/config" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("token") != "test-token" {
			t.Fatalf("missing token param")
		}
		w.Header().Set("ETag", `"config-etag-v1"`)
		json.NewEncoder(w).Encode(cfg)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", 1, "vmess", newTestLogger())
	result, err := client.FetchConfig()
	if err != nil {
		t.Fatalf("FetchConfig failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil config")
	}
	if result.ServerPort != 443 {
		t.Errorf("expected ServerPort 443, got %d", result.ServerPort)
	}
	if result.BaseConfig.PushInterval != 60 {
		t.Errorf("expected PushInterval 60, got %d", result.BaseConfig.PushInterval)
	}
	if client.configETag != `"config-etag-v1"` {
		t.Errorf("expected configETag to be stored, got %q", client.configETag)
	}
}

func TestFetchConfig_ETag304(t *testing.T) {
	etag := `"config-etag-v1"`
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		json.NewEncoder(w).Encode(NodeConfig{ServerPort: 443})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", 1, "vmess", newTestLogger())

	// First call: should get config
	result, err := client.FetchConfig()
	if err != nil {
		t.Fatalf("first FetchConfig failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil config on first call")
	}

	// Second call: should get 304 (nil config)
	result, err = client.FetchConfig()
	if err != nil {
		t.Fatalf("second FetchConfig failed: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil config on 304")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestFetchUsers_Success(t *testing.T) {
	speedLimit := 100
	users := []User{
		{ID: 1, UUID: "uuid-1", SpeedLimit: &speedLimit},
		{ID: 2, UUID: "uuid-2", DeviceLimit: nil},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/server/UniProxy/user" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("ETag", `"user-etag-v1"`)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": users,
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", 1, "vmess", newTestLogger())
	result, err := client.FetchUsers()
	if err != nil {
		t.Fatalf("FetchUsers failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result))
	}
	if result[0].UUID != "uuid-1" {
		t.Errorf("expected uuid-1, got %s", result[0].UUID)
	}
	if result[0].SpeedLimit == nil || *result[0].SpeedLimit != 100 {
		t.Error("expected SpeedLimit 100")
	}
	if result[1].DeviceLimit != nil {
		t.Error("expected nil DeviceLimit")
	}
	if client.userETag != `"user-etag-v1"` {
		t.Errorf("expected userETag stored, got %q", client.userETag)
	}
}

func TestFetchUsers_ETag304(t *testing.T) {
	etag := `"user-etag-v1"`
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": []User{{ID: 1, UUID: "test-uuid"}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", 1, "vmess", newTestLogger())

	// First call: should get users
	result, err := client.FetchUsers()
	if err != nil {
		t.Fatalf("first FetchUsers failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 user, got %d", len(result))
	}

	// Second call: should get 304 (nil)
	result, err = client.FetchUsers()
	if err != nil {
		t.Fatalf("second FetchUsers failed: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil users on 304")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestFetchConfig_ETagWithQuotes(t *testing.T) {
	// Verify ETag with double quotes is stored and sent as-is
	etag := `"abc123"`
	var receivedETag string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedETag = r.Header.Get("If-None-Match")
		w.Header().Set("ETag", etag)
		json.NewEncoder(w).Encode(NodeConfig{ServerPort: 443})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", 1, "vmess", newTestLogger())

	// First call: no If-None-Match
	_, _ = client.FetchConfig()
	if receivedETag != "" {
		t.Errorf("first call should not send If-None-Match, got %q", receivedETag)
	}

	// Second call: should send stored ETag with quotes
	_, _ = client.FetchConfig()
	if receivedETag != etag {
		t.Errorf("expected If-None-Match %q, got %q", etag, receivedETag)
	}
}

func TestTestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(NodeConfig{ServerPort: 443})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", 1, "vmess", newTestLogger())
	err := client.TestConnection()
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
}

func TestTestConnection_Failure(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", 1, "vmess", newTestLogger())
	err := client.TestConnection()
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestFetchConfig_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", 1, "vmess", newTestLogger())
	result, err := client.FetchConfig()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if result != nil {
		t.Fatal("expected nil result on error")
	}
}

func TestFetchUsers_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", 1, "vmess", newTestLogger())
	result, err := client.FetchUsers()
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if result != nil {
		t.Fatal("expected nil result on error")
	}
}
