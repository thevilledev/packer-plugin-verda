packer {
  required_plugins {
    verda = {
      version = ">= 0.1.0"
      source  = "github.com/thevilledev/verda"
    }
  }
}

source "verda-instance" "ubuntu" {
  instance_type = "1L40S.20V"
  location_code = "FIN-02"
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
