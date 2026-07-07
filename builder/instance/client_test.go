package instance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestSDKClientCloneVolumeSendsLocationAndType(t *testing.T) {
	var clonePayload volumeCloneActionRequest

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			if r.Method != http.MethodPost {
				return nil, fmt.Errorf("token method = %s", r.Method)
			}
			return jsonResponse(r, http.StatusOK, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`), nil
		case "/v1/volumes":
			if r.Method != http.MethodPut {
				return nil, fmt.Errorf("clone method = %s", r.Method)
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
