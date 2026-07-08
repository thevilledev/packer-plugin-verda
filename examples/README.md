## packer-plugin-verda examples

Contains Packer templates to show how the plugin can be used.

NOTE: Currently the Verda Terraform provider requires a custom build to work
with these templates. You can follow an upstream issue at https://github.com/verda-cloud/terraform-provider-verda/issues/16.

Until then, I've prepared a fix branch. If you're ready to build your own provider, follow these steps:

1. Clone the provider and fix branch from my fork:

```sh
git clone https://github.com/thevilledev/terraform-provider-verda.git
cd terraform-provider-verda
git checkout fix/instance-state-inconsistency
```

You can validate the changes with `git log`.

2. Build the provider:

```sh
mkdir -p /tmp/verda-provider
go build -o /tmp/verda-provider/terraform-provider-verda .
```

3. Create a Terraform CLI config override:

To `/tmp/verda-provider/verda-dev.tfrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/thevilledev/verda" = "/tmp/verda-provider"
  }

  direct {}
}
```

This tells Terraform to use the local provider binary instead of downloading the published registry version.

4. Keep the normal Terraform provider block

Your Terraform config can stay as-is:

```hcl
terraform {
  required_providers {
    verda = {
      source  = "github.com/thevilledev/verda"
      version = ">= 1.1.2"
    }
  }
}
```

The dev override replaces the provider at runtime.

5. Init Terraform normally

For example, in this directory:

```sh
terraform init
```

6. Run with the custom provider:

```sh
TF_CLI_CONFIG_FILE=/tmp/verda-dev.tfrc terraform plan
TF_CLI_CONFIG_FILE=/tmp/verda-dev.tfrc terraform apply
```
