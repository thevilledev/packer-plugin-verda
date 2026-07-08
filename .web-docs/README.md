# Packer Plugin Verda

The Verda Packer plugin provides a builder for creating Verda Cloud instances and reusable OS volume artifacts.

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

Run `packer init .` before building.

## Components

### Builders

- [verda-instance](/packer/integrations/thevilledev/verda/latest/components/builder/instance) - Creates a Verda instance, runs provisioners, and returns either an instance artifact or a cloned OS volume artifact.
