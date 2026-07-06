package instance

import (
	"context"
	"fmt"
	"net/http"

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
	CloneVolume(context.Context, string, verda.VolumeCloneRequest) (string, error)
	DeleteVolume(context.Context, string, bool) error
	CreateSSHKey(context.Context, verda.CreateSSHKeyRequest) (*verda.SSHKey, error)
	DeleteSSHKey(context.Context, string) error
	CreateStartupScript(context.Context, verda.CreateStartupScriptRequest) (*verda.StartupScript, error)
	DeleteStartupScript(context.Context, string) error
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

func (c sdkClient) CloneVolume(ctx context.Context, id string, req verda.VolumeCloneRequest) (string, error) {
	return c.client.Volumes.CloneVolume(ctx, id, req)
}

func (c sdkClient) DeleteVolume(ctx context.Context, id string, force bool) error {
	return c.client.Volumes.DeleteVolume(ctx, id, force)
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
