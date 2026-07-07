packer {
  required_plugins {
    verda = {
      version = ">= 0.1.0"
      source  = "github.com/thevilledev/verda"
    }
  }
}

variable "authorized_keys_file" {
  type        = string
  description = "Path to an authorized_keys-format file to bake into the OS volume for later boots."
}

source "verda-instance" "ubuntu_volume" {
  instance_type = "CPU.4V.16G"
  location_code = "FIN-03"
  image         = "ubuntu-24.04"
  hostname      = "packer-verda-example"

  ssh_username              = "root"
  ssh_clear_authorized_keys = true

  artifact_type                  = "os_volume"
  artifact_volume_name           = "packer-verda-volume-root"
  artifact_volume_location_codes = ["FIN-02"]
}

build {
  sources = ["source.verda-instance.ubuntu_volume"]

  provisioner "file" {
    source      = var.authorized_keys_file
    destination = "/tmp/verda_authorized_keys"
  }

  provisioner "shell" {
    inline = [
      "cloud-init status --wait || true",
      "install -d -m 0700 /root/.ssh",
      "touch /root/.ssh/authorized_keys",
      "chmod 0600 /root/.ssh/authorized_keys",
      "while IFS= read -r key; do [ -n \"$key\" ] || continue; grep -qxF \"$key\" /root/.ssh/authorized_keys || printf '%s\\n' \"$key\" >> /root/.ssh/authorized_keys; done < /tmp/verda_authorized_keys",
      "chown root:root /root/.ssh /root/.ssh/authorized_keys",
      "rm -f /tmp/verda_authorized_keys",
      "apt-get update",
    ]
  }
}
