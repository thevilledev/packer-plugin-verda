# Packer Plugin Verda

`packer-plugin-verda` provides a Packer builder for Verda Cloud.

The `verda-instance` builder creates a Verda instance, waits for it to become reachable, runs provisioners, and returns either the created instance or a cloned OS volume artifact.

## Installation

```hcl
packer {
  required_plugins {
    verda = {
      version = ">= 0.1.0"
      source  = "github.com/thevilledev/verda"
    }
  }
}
```

Then run:

```sh
packer init .
```

For local development or private repositories, install the built binary directly:

```sh
make build
packer plugins install --path ./packer-plugin-verda github.com/thevilledev/verda
```

## Usage

Set credentials with `VERDA_CLIENT_ID` and `VERDA_CLIENT_SECRET`, or pass `client_id` and `client_secret` in the source block.

```hcl
source "verda-instance" "ubuntu" {
  instance_type = "1L40S.20V"
  location_code = "FIN-03"
  image         = "ubuntu-24.04"
  hostname      = "packer-verda-example"

  ssh_username = "root"
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

To produce a reusable OS volume instead of an instance artifact, set `artifact_type = "os_volume"`. The artifact ID can then be used as the Verda instance `image` value in Terraform or another API client.

## Documentation

- Builder reference: [docs/builders/instance.mdx](docs/builders/instance.mdx)
- Examples: [examples/basic.pkr.hcl](examples/basic.pkr.hcl), [examples/os-volume.pkr.hcl](examples/os-volume.pkr.hcl)

The option reference is generated from the builder config comments with `packer-sdc`, matching the pattern used by other Packer plugins.

## Development

```sh
make tidy
make generate
make test
make lint
```

Render the docs locally with:

```sh
make renderdocs
```

Build the GitHub Pages site input with:

```sh
make docs-site
```

## Release

Packer remote plugin installation expects GitHub release assets named with the plugin API version and a SHA256SUMS file.

```sh
make snapshot
git tag -a v0.1.0 -m v0.1.0
git push origin v0.1.0
```

## License

MIT.
