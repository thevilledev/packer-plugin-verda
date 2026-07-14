# Changelog

All notable changes to this project will be documented in this file.

This project follows semantic versioning for Packer plugin releases.

## Unreleased

- Align repository metadata, CI, docs generation, and release packaging with established Packer plugin conventions.
- Correct artifact lifecycle, cancellation, validation, and cleanup behavior, and expand lifecycle regression coverage.
- Print and persist every multi-location OS volume artifact ID while retaining the detached source-location clone required for cross-datacenter cloning.
- Treat `contract = "SPOT"` and `is_spot = true` as equivalent spot build requests, and document spot build usage.
