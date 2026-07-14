package instance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	pluginVersion "github.com/thevilledev/packer-plugin-verda/version"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestSDKClientCloneVolumeSendsLocationAndType(t *testing.T) {
	var clonePayload volumeCloneActionRequest
	expectedUserAgent := verda.BuildUserAgent(pluginVersion.UserAgent())

	httpClient := newUserAgentTestHTTPClient(expectedUserAgent, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			if r.Method != http.MethodPost {
				return nil, fmt.Errorf("token method = %s", r.Method)
			}
			if got := r.Header.Get("User-Agent"); got != expectedUserAgent {
				return nil, fmt.Errorf("token User-Agent = %q", got)
			}
			return jsonResponse(r, http.StatusOK, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`), nil
		case "/v1/volumes":
			if r.Method != http.MethodPut {
				return nil, fmt.Errorf("clone method = %s", r.Method)
			}
			if got := r.Header.Get("User-Agent"); got != expectedUserAgent {
				return nil, fmt.Errorf("clone User-Agent = %q", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				return nil, fmt.Errorf("Authorization = %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&clonePayload); err != nil {
				return nil, fmt.Errorf("decode clone payload: %w", err)
			}
			return jsonResponse(r, http.StatusCreated, `{"id":"vol-cloned"}`), nil
		default:
			return nil, fmt.Errorf("unexpected path %q", r.URL.Path)
		}
	}))

	verdaSDK, err := verda.NewClient(
		verda.WithClientID("client-id"),
		verda.WithClientSecret("client-secret"),
		verda.WithBaseURL("https://verda.test/v1"),
		verda.WithUserAgent(pluginVersion.UserAgent()),
		verda.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	client := sdkClient{client: verdaSDK}
	id, err := client.CloneVolume(context.Background(), "vol-source", volumeCloneRequest{
		Name:         "artifact-volume",
		LocationCode: "FIN-01",
		Type:         verda.VolumeTypeNVMe,
	})
	if err != nil {
		t.Fatalf("CloneVolume: %v", err)
	}
	if id != "vol-cloned" {
		t.Fatalf("cloned volume ID = %q", id)
	}

	if clonePayload.ID != "vol-source" ||
		clonePayload.Action != verda.VolumeActionClone ||
		clonePayload.Name != "artifact-volume" ||
		clonePayload.LocationCode != "FIN-01" ||
		clonePayload.Type != verda.VolumeTypeNVMe {
		t.Fatalf("clone payload = %#v", clonePayload)
	}
}

func TestNewSDKClientConfiguresUserAgent(t *testing.T) {
	verdaSDK, err := newSDKClient(&Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		APITimeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("newSDKClient: %v", err)
	}

	if got, want := verdaSDK.UserAgent, pluginVersion.UserAgent(); got != want {
		t.Fatalf("UserAgent = %q, want %q", got, want)
	}

	transport, ok := verdaSDK.HTTPClient.Transport.(userAgentTransport)
	if !ok {
		t.Fatalf("HTTPClient.Transport = %T, want userAgentTransport", verdaSDK.HTTPClient.Transport)
	}
	if got, want := transport.userAgent, verda.BuildUserAgent(pluginVersion.UserAgent()); got != want {
		t.Fatalf("transport User-Agent = %q, want %q", got, want)
	}
}

func TestSDKClientCreateSSHKeySendsUserAgentOnDirectSDKRequest(t *testing.T) {
	expectedUserAgent := verda.BuildUserAgent(pluginVersion.UserAgent())
	seen := map[string]string{}

	httpClient := newUserAgentTestHTTPClient(expectedUserAgent, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen[r.Method+" "+r.URL.Path] = r.Header.Get("User-Agent")

		switch r.URL.Path {
		case "/v1/oauth2/token":
			if r.Method != http.MethodPost {
				return nil, fmt.Errorf("token method = %s", r.Method)
			}
			return jsonResponse(r, http.StatusOK, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`), nil
		case "/v1/ssh-keys":
			if r.Method != http.MethodPost {
				return nil, fmt.Errorf("create SSH key method = %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				return nil, fmt.Errorf("Authorization = %q", got)
			}
			return jsonResponse(r, http.StatusCreated, `key-1`), nil
		case "/v1/ssh-keys/key-1":
			if r.Method != http.MethodGet {
				return nil, fmt.Errorf("get SSH key method = %s", r.Method)
			}
			return jsonResponse(r, http.StatusOK, `[{"id":"key-1","name":"test-key","key":"ssh-rsa AAA"}]`), nil
		default:
			return nil, fmt.Errorf("unexpected path %q", r.URL.Path)
		}
	}))

	verdaSDK, err := verda.NewClient(
		verda.WithClientID("client-id"),
		verda.WithClientSecret("client-secret"),
		verda.WithBaseURL("https://verda.test/v1"),
		verda.WithUserAgent(pluginVersion.UserAgent()),
		verda.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	client := sdkClient{client: verdaSDK}
	key, err := client.CreateSSHKey(context.Background(), verda.CreateSSHKeyRequest{
		Name:      "test-key",
		PublicKey: "ssh-rsa AAA",
	})
	if err != nil {
		t.Fatalf("CreateSSHKey: %v", err)
	}
	if key.ID != "key-1" {
		t.Fatalf("SSH key ID = %q", key.ID)
	}

	for _, request := range []string{
		"POST /v1/oauth2/token",
		"POST /v1/ssh-keys",
		"GET /v1/ssh-keys/key-1",
	} {
		if got := seen[request]; got != expectedUserAgent {
			t.Fatalf("%s User-Agent = %q, want %q", request, got, expectedUserAgent)
		}
	}
}

func TestUserAgentTransportPreservesExistingUserAgent(t *testing.T) {
	const existingUserAgent = "custom-client/1.0"

	httpClient := &http.Client{
		Transport: userAgentTransport{
			base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if got := r.Header.Get("User-Agent"); got != existingUserAgent {
					return nil, fmt.Errorf("User-Agent = %q", got)
				}
				return jsonResponse(r, http.StatusOK, `{}`), nil
			}),
			userAgent: "packer-plugin-verda/test",
		},
	}

	req, err := http.NewRequest(http.MethodGet, "https://verda.test/v1/ping", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("User-Agent", existingUserAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()
}

func TestParseVolumeCloneResponse(t *testing.T) {
	tests := map[string]string{
		`{"id":"vol-object"}`: "vol-object",
		`["vol-array"]`:       "vol-array",
		`"vol-string"`:        "vol-string",
	}
	for body, want := range tests {
		got, err := parseVolumeCloneResponse(json.RawMessage(body))
		if err != nil {
			t.Fatalf("parseVolumeCloneResponse(%s): %v", body, err)
		}
		if got != want {
			t.Fatalf("parseVolumeCloneResponse(%s) = %q, want %q", body, got, want)
		}
	}
}

func TestParseVolumeCloneResponseRejectsMissingID(t *testing.T) {
	for _, body := range []string{"", `{}`, `[]`, `""`, `null`, `{"id":""}`} {
		t.Run(body, func(t *testing.T) {
			if _, err := parseVolumeCloneResponse(json.RawMessage(body)); err == nil {
				t.Fatalf("parseVolumeCloneResponse(%q) returned nil error", body)
			}
		})
	}
}

func TestSDKClientCloneVolumeRequiresName(t *testing.T) {
	client := sdkClient{}
	if _, err := client.CloneVolume(context.Background(), "vol-source", volumeCloneRequest{}); err == nil {
		t.Fatal("expected missing clone name to fail")
	}
}

func TestSDKClientRetriesUnauthorizedWithFreshToken(t *testing.T) {
	tokenRequests := 0
	volumeRequests := 0

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			tokenRequests++
			token := "expired-token"
			if tokenRequests > 1 {
				token = "fresh-token"
			}
			return jsonResponse(r, http.StatusOK, fmt.Sprintf(`{"access_token":%q,"token_type":"Bearer","expires_in":3600}`, token)), nil
		case "/v1/volumes/vol-source":
			volumeRequests++
			switch r.Header.Get("Authorization") {
			case "Bearer expired-token":
				return jsonResponse(r, http.StatusUnauthorized, `{"code":"unauthorized_request","message":"Access token is missing or invalid"}`), nil
			case "Bearer fresh-token":
				return jsonResponse(r, http.StatusOK, `{"id":"vol-source","name":"source","type":"NVMe","status":"detached","location":"FIN-03"}`), nil
			default:
				return nil, fmt.Errorf("Authorization = %q", r.Header.Get("Authorization"))
			}
		default:
			return nil, fmt.Errorf("unexpected path %q", r.URL.Path)
		}
	})}

	verdaSDK, err := verda.NewClient(
		verda.WithClientID("client-id"),
		verda.WithClientSecret("client-secret"),
		verda.WithBaseURL("https://verda.test/v1"),
		verda.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	client := sdkClient{client: verdaSDK}
	volume, err := client.GetVolume(context.Background(), "vol-source")
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if volume.ID != "vol-source" {
		t.Fatalf("volume ID = %q", volume.ID)
	}
	if tokenRequests != 2 || volumeRequests != 2 {
		t.Fatalf("tokenRequests = %d, volumeRequests = %d", tokenRequests, volumeRequests)
	}
}

func newUserAgentTestHTTPClient(userAgent string, transport http.RoundTripper) *http.Client {
	return &http.Client{Transport: userAgentTransport{
		base:      transport,
		userAgent: userAgent,
	}}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(req *http.Request, statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
