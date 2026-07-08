# For full specification on the configuration of this file visit:
# https://github.com/hashicorp/integration-template#metadata-configuration
integration {
  name        = "Verda"
  description = "The Verda plugin can be used with HashiCorp Packer to create Verda Cloud instances and reusable OS volume artifacts."
  identifier  = "packer/thevilledev/verda"

  docs {
    process_docs    = true
    readme_location = "./README.md"
    external_url    = "https://github.com/thevilledev/packer-plugin-verda"
  }

  license {
    type = "MIT"
    url  = "https://github.com/thevilledev/packer-plugin-verda/blob/main/LICENSE"
  }

  component {
    type = "builder"
    name = "Verda Instance"
    slug = "instance"
  }
}
