# Packer Plugin Verda

`packer-plugin-verda` provides a Packer builder for Verda Cloud.

The first builder is `verda.instance`. It creates a Verda instance, waits for it to become reachable, and runs standard Packer provisioners over SSH. By default it returns the created instance as the build artifact.

For image-like workflows, set `artifact_type = "os_volume"`. The builder will shut down the provisioned instance, clone its OS volume, and return the cloned volume as the artifact. The Verda API accepts a previously customized OS volume ID in the instance `image` field, so that volume ID can be used later by Terraform or another API client as the source OS disk for new instances. For multi-location fleets, clone the Packer artifact to each target Verda location and select the matching volume ID when launching each instance.

## Requirements

- Go 1.26+
- Packer 1.10+
- Verda client credentials

## Build

```sh
make build
packer plugins install --path ./packer-plugin-verda github.com/thevilledev/verda
packer build examples/basic.pkr.hcl
```

For private repositories, prefer the manual install path above. `packer init` queries GitHub for release tags and assets; for a private repository, that requires `PACKER_GITHUB_API_TOKEN` with access to the repo and a GitHub release that includes Packer plugin artifacts and SHA256SUM files.

## Configuration

Credentials can be provided with `client_id` and `client_secret`, or through `VERDA_CLIENT_ID` and `VERDA_CLIENT_SECRET`.

```hcl
packer {
  required_plugins {
    verda = {
      version = ">= 0.1.0"
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
- `artifact_volume_location_code` controls the single cloned OS volume location.
- `artifact_volume_location_codes` clones the OS volume to multiple locations. The first location is the primary artifact ID, and generated data exposes all volume IDs by location.
- `skip_shutdown_before_artifact = true` skips the default shutdown before OS volume capture.
- `keep_instance = true` keeps the created instance after the build. The default is to delete it during cleanup.
- `volume_ids_to_delete` controls which volumes are deleted when the instance is deleted.
- Verda volumes do not expose tags, labels, or arbitrary metadata in the public API, Terraform provider, or Go SDK. Use descriptive volume names or external inventory when you need release metadata.

## Terraform Handoff

When `artifact_type = "os_volume"`, the Packer artifact ID is the primary cloned OS volume ID. For one derived instance, pass that ID or a clone of it to Terraform as the Verda instance `image`; do not put it in `existing_volumes`, which is only for additional non-OS volumes.

For multiple locations, set `artifact_volume_location_codes` during the Packer build and store the generated `VolumeIDsByLocation` map. For multiple instances in the same location, clone that location's Packer artifact once per instance, then pass each clone ID as that instance's `image`. The current Verda Terraform provider can consume the OS volume ID, but does not expose a first-class volume clone resource, so create per-instance clone IDs through the API, Go SDK, or a small pre-Terraform step.

```hcl
variable "os_volume_ids_by_location" {
  description = "Packer VolumeIDsByLocation generated data."
  type        = map(string)
}

resource "verda_instance" "app" {
  instance_type = "1B200.30V"
  image         = var.os_volume_ids_by_location["FIN-03"]
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

## Release

Packer remote plugin installation expects GitHub release assets named with the plugin API version and a SHA256SUMS file. This repository uses GoReleaser, following the same release asset pattern as `digitalocean/packer-plugin-digitalocean`.

```sh
make snapshot
git tag -a v0.1.0 -m v0.1.0
git push origin v0.1.0
```

The tag push runs GoReleaser in GitHub Actions and uploads archives such as `packer-plugin-verda_v0.1.0_x5.0_linux_amd64.zip` plus `packer-plugin-verda_v0.1.0_SHA256SUMS`.
