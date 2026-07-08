# Contributing

## Development

Run the standard checks before opening a pull request:

```sh
make tidy-check
make check-fmt
make check-generate
make test
make lint
make plugin-check
```

Generated HCL2 specs and option documentation are committed. When builder configuration changes, run:

```sh
make generate
```

## Acceptance Tests

Acceptance tests create real Verda resources and may incur cost. They are disabled by default.

```sh
PACKER_ACC=1 \
VERDA_CLIENT_ID=... \
VERDA_CLIENT_SECRET=... \
VERDA_ACC_INSTANCE_TYPE=... \
VERDA_ACC_IMAGE=... \
make testacc
```

`VERDA_ACC_LOCATION_CODE`, `VERDA_ACC_HOSTNAME`, and `VERDA_ACC_BASE_URL` are optional.
