package instance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/verda-cloud/packer-plugin-verda/version"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

type verdaClient interface {
	GetInstance(context.Context, string) (*verda.Instance, error)
	CreateInstance(context.Context, verda.CreateInstanceRequest) (*verda.Instance, error)
	DeleteInstance(context.Context, string, []string, bool) error
	ShutdownInstance(context.Context, string) error
	GetVolume(context.Context, string) (*verda.Volume, error)
	CloneVolume(context.Context, string, volumeCloneRequest) (string, error)
	DeleteVolume(context.Context, string, bool) error
	CreateSSHKey(context.Context, verda.CreateSSHKeyRequest) (*verda.SSHKey, error)
	DeleteSSHKey(context.Context, string) error
	CreateStartupScript(context.Context, verda.CreateStartupScriptRequest) (*verda.StartupScript, error)
	DeleteStartupScript(context.Context, string) error
}

type volumeCloneRequest struct {
	Name         string
	LocationCode string
	Type         string
}

type volumeCloneActionRequest struct {
	ID           string `json:"id"`
	Action       string `json:"action"`
	Name         string `json:"name"`
	Type         string `json:"type,omitempty"`
	LocationCode string `json:"location_code,omitempty"`
}

type sdkClient struct {
	client *verda.Client
}

func newSDKClient(config *Config) (*verda.Client, error) {
	options := []verda.ClientOption{
		verda.WithClientID(config.ClientID),
		verda.WithClientSecret(config.ClientSecret),
		verda.WithUserAgent(fmt.Sprintf("packer-plugin-verda/%s", version.Version)),
		verda.WithHTTPClient(&http.Client{Timeout: config.APITimeout}),
		verda.WithDebugLogging(config.Debug),
	}
	if config.BaseURL != "" {
		options = append(options, verda.WithBaseURL(config.BaseURL))
	}
	return verda.NewClient(options...)
}

func (c sdkClient) GetInstance(ctx context.Context, id string) (*verda.Instance, error) {
	return c.client.Instances.GetByID(ctx, id)
}

func (c sdkClient) CreateInstance(ctx context.Context, req verda.CreateInstanceRequest) (*verda.Instance, error) {
	return c.client.Instances.Create(ctx, req)
}

func (c sdkClient) DeleteInstance(ctx context.Context, id string, volumeIDs []string, deletePermanently bool) error {
	return c.client.Instances.Delete(ctx, []string{id}, volumeIDs, deletePermanently)
}

func (c sdkClient) ShutdownInstance(ctx context.Context, id string) error {
	return c.client.Instances.Shutdown(ctx, id)
}

func (c sdkClient) GetVolume(ctx context.Context, id string) (*verda.Volume, error) {
	return c.client.Volumes.GetVolume(ctx, id)
}

func (c sdkClient) CloneVolume(ctx context.Context, id string, req volumeCloneRequest) (string, error) {
	if strings.TrimSpace(req.Name) == "" {
		return "", fmt.Errorf("volume clone name is required")
	}

	bodyBytes, err := json.Marshal(volumeCloneActionRequest{
		ID:           id,
		Action:       verda.VolumeActionClone,
		Name:         req.Name,
		Type:         req.Type,
		LocationCode: req.LocationCode,
	})
	if err != nil {
		return "", fmt.Errorf("marshaling volume clone request: %w", err)
	}

	httpReq, err := c.client.NewRequest(ctx, http.MethodPut, "/volumes", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	var body json.RawMessage
	if _, err := c.client.Do(httpReq, &body); err != nil {
		return "", err
	}
	return parseVolumeCloneResponse(body)
}

func (c sdkClient) DeleteVolume(ctx context.Context, id string, force bool) error {
	return c.client.Volumes.DeleteVolume(ctx, id, force)
}

func parseVolumeCloneResponse(body json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", fmt.Errorf("no volume ID returned from clone operation")
	}

	var objectResponse struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(trimmed, &objectResponse); err == nil && objectResponse.ID != "" {
		return objectResponse.ID, nil
	}

	var arrayResponse []string
	if err := json.Unmarshal(trimmed, &arrayResponse); err == nil && len(arrayResponse) > 0 && arrayResponse[0] != "" {
		return arrayResponse[0], nil
	}

	var stringResponse string
	if err := json.Unmarshal(trimmed, &stringResponse); err == nil && stringResponse != "" {
		return stringResponse, nil
	}

	return "", fmt.Errorf("no volume ID returned from clone operation")
}

func (c sdkClient) CreateSSHKey(ctx context.Context, req verda.CreateSSHKeyRequest) (*verda.SSHKey, error) {
	return c.client.SSHKeys.AddSSHKey(ctx, &req)
}

func (c sdkClient) DeleteSSHKey(ctx context.Context, id string) error {
	return c.client.SSHKeys.DeleteSSHKey(ctx, id)
}

func (c sdkClient) CreateStartupScript(ctx context.Context, req verda.CreateStartupScriptRequest) (*verda.StartupScript, error) {
	return c.client.StartupScripts.AddStartupScript(ctx, &req)
}

func (c sdkClient) DeleteStartupScript(ctx context.Context, id string) error {
	return c.client.StartupScripts.DeleteStartupScript(ctx, id)
}

func putClient(state multistep.StateBag, client verdaClient) {
	state.Put(stateKeyClient, client)
}

func getClient(state multistep.StateBag) verdaClient {
	client, _ := state.Get(stateKeyClient).(verdaClient)
	return client
}
