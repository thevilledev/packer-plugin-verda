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

  keep_instance = true
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
