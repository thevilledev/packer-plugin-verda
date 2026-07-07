# Packer Plugin Verda

`packer-plugin-verda` provides a Packer builder for Verda Cloud.

The first builder is `verda.instance`. It creates a Verda instance, waits for it to become reachable, and runs standard Packer provisioners over SSH. By default it returns the created instance as the build artifact.

For image-like workflows, set `artifact_type = "os_volume"`. The builder will shut down the provisioned instance, clone its OS volume, and return the cloned volume as the artifact. The Verda API accepts a previously customized OS volume ID in the instance `image` field, so that volume ID can be used later by Terraform or another API client as the source OS disk for new instances. For fleets, keep the Packer artifact as a golden volume and clone it once per instance before launch.

## Requirements

- Go 1.26+
- Packer 1.10+
- Verda client credentials

## Build

```sh
make build
packer plugins install --path ./packer-plugin-verda github.com/thevilledev/verda
```

## Configuration

Credentials can be provided with `client_id` and `client_secret`, or through `VERDA_CLIENT_ID` and `VERDA_CLIENT_SECRET`.

```hcl
packer {
  required_plugins {
    verda = {
      version = ">= 0.0.1"
      source  = "github.com/thevilledev/verda"
    }
  }
}

source "verda-instance" "ubuntu" {
  instance_type = "V100"
  image         = "ubuntu-24.04"
  hostname      = "packer-verda-example"

  ssh_username = "root"

  keep_instance      = true
  delete_permanently = false
}

build {
  sources = ["source.verda-instance.ubuntu"]

  provisioner "shell" {
    inline = [
      "cloud-init status --wait || true",
      "uname -a",
    ]
  }
}
```

## Important Options

- `instance_type`, `image`, and `hostname` are required.
- `location_code` defaults to `FIN-03`.
- `client_id` and `client_secret` default from `VERDA_CLIENT_ID` and `VERDA_CLIENT_SECRET`.
- `communicator` supports `ssh` and `none`; SSH defaults to user `root`.
- By default the builder creates a temporary Verda SSH key from Packer's generated SSH key and deletes it during cleanup.
- Set `skip_temporary_ssh_key = true` and pass `ssh_key_ids` to use existing Verda SSH keys.
- `artifact_type = "os_volume"` returns a cloned OS volume artifact instead of an instance artifact.
- `clone_os_volume` defaults to `true`; set it to `false` to use the source OS volume directly.
- `artifact_volume_name` controls the cloned OS volume name.
- `skip_shutdown_before_artifact = true` skips the default shutdown before OS volume capture.
- `keep_instance = true` keeps the created instance after the build. The default is to delete it during cleanup.
- `volume_ids_to_delete` controls which volumes are deleted when the instance is deleted.
- Verda volumes do not expose tags, labels, or arbitrary metadata in the public API, Terraform provider, or Go SDK. Use descriptive volume names or external inventory when you need release metadata.

## Terraform Handoff

When `artifact_type = "os_volume"`, the Packer artifact ID is the cloned OS volume ID. For one derived instance, pass that ID or a clone of it to Terraform as the Verda instance `image`; do not put it in `existing_volumes`, which is only for additional non-OS volumes.

For multiple instances, clone the Packer artifact once per instance, then pass each clone ID as that instance's `image`. The current Verda Terraform provider can consume the OS volume ID, but does not expose a first-class volume clone resource, so create clone IDs through the API, Go SDK, or a small pre-Terraform step.

```hcl
variable "os_volume_id" {
  description = "Packer artifact ID or a per-instance clone of it."
  type = string
}

resource "verda_instance" "app" {
  instance_type = "1B200.30V"
  image         = var.os_volume_id
  hostname      = "app-01"
  description   = "App server from Packer OS volume"
  location      = "FIN-03"

  ssh_key_ids = [verda_ssh_key.main.id]
}
```

## Development

```sh
make tidy
make test
make lint
```
