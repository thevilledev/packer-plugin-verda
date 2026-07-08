package instance

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/packer"
)

func TestAccBuilderInstanceLifecycleNoneCommunicator(t *testing.T) {
	if os.Getenv("PACKER_ACC") == "" {
		t.Skip("set PACKER_ACC=1 to run Verda acceptance tests")
	}

	instanceType := requireAccEnv(t, "VERDA_ACC_INSTANCE_TYPE")
	image := requireAccEnv(t, "VERDA_ACC_IMAGE")
	locationCode := firstNonEmpty(os.Getenv("VERDA_ACC_LOCATION_CODE"), defaultLocationCode)
	hostname := firstNonEmpty(os.Getenv("VERDA_ACC_HOSTNAME"), "packer-acc-"+time.Now().UTC().Format("20060102150405"))

	var b Builder
	config := map[string]interface{}{
		"instance_type": instanceType,
		"image":         image,
		"hostname":      hostname,
		"location_code": locationCode,
		"communicator":  "none",
	}
	if baseURL := os.Getenv("VERDA_ACC_BASE_URL"); baseURL != "" {
		config["base_url"] = baseURL
	}

	if _, _, err := b.Prepare(config); err != nil {
		t.Fatalf("Prepare returned error: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	artifact, err := b.Run(ctx, packer.TestUi(t), &packer.MockHook{})
	if err != nil {
		t.Fatalf("Run returned error: %s", err)
	}
	if artifact == nil {
		t.Fatal("Run returned nil artifact")
	}
	if artifact.Id() == "" {
		t.Fatal("artifact ID is empty")
	}
}

func requireAccEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s must be set for acceptance tests", name)
	}
	return value
}
