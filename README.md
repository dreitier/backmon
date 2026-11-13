# backmon - Monitoring of backup files in filesystems and object storages

[![CI](https://github.com/dreitier/backmon/actions/workflows/release.yml/badge.svg?branch=main)](https://github.com/dreitier/backmon/actions/workflows/release.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/dreitier/backmon/badge)](https://scorecard.dev/viewer/?uri=github.com/dreitier/backmon)

*backmon* monitors backup files inside a filesystem, an S3-compatible object storage like AWS S3 or MinIO, or Google Cloud Storage.

## Description

With *backmon* you can monitor and check the presence, size and timestamps of your recurring backup files. Your backup files can be stored in a local filesystem, inside an S3-compatible object storage like AWS S3 or MinIO, or in Google Cloud Storage buckets.

You can easily integrate *backmon* into your Prometheus- and Grafana-based infrastructure for analysing the duration of creating backups or alerting if a backup fails some constraints.

## Getting started
### Documentation
You can find our official documentation at [https://dreitier.github.io/backmon-docs](https://dreitier.github.io/backmon-docs).

### Storage
Each environment of the `config.yaml` uses one storage; see [`config.yaml.template`](config.yaml.template) for a complete example.

| Storage                                 | Section | Disks                                 |
|-----------------------------------------|---------|---------------------------------------|
| Local filesystem                        | `path`  | the configured directory              |
| AWS S3, MinIO and other S3-compatibles  | `s3`    | each bucket                           |
| Google Cloud Storage                    | `gcs`   | each bucket of the configured project |

```yaml
environments:
  gcs-backups:
    gcs:
      project_id: my-gcp-project
      # optional; without it, Application Default Credentials are used (e.g. Workload Identity on GKE)
      credentials_file: /etc/backmon/gcs-service-account.json
```

For Google Cloud Storage, *backmon* needs the permissions `storage.buckets.list` (in the project, for `auto_discover_disks`), `storage.objects.list` and `storage.objects.get`, and `storage.objects.delete` if backups are purged.

### API
Besides `/metrics`, *backmon* serves a read-only API below `/api`. It is protected by HTTP Basic Auth if `http.basic_auth` is configured.

The API follows [HATEOAS](https://en.wikipedia.org/wiki/HATEOAS) and responds with [HAL](https://datatracker.ietf.org/doc/html/draft-kelly-json-hal) documents (`application/hal+json`): clients start at `/api` and follow the links of the responses instead of building URLs themselves.

| Resource | Path | Embedded resources |
| --- | --- | --- |
| Entry point | `/api` | `disks` |
| Disk | `/api/{disk}` | `directories` |
| Directory | `/api/{disk}/{directory}` | `files` |
| File | `/api/{disk}/{directory}/{file}` | `variants` |

Each resource has a `self` link and, except for the entry point, an `up` link to its parent. Its children are listed in `_embedded` with their `name` and their own `self` link. The variants of a file link to the newest backup with `download`, if `downloads.enabled` is set. Names are percent-encoded in links, e.g. the slashes in the name of a local disk.

```
$ curl -u my_username:my_password http://localhost:8000/api/my-bucket
```

```json
{
  "name": "my-bucket",
  "quota": 0,
  "_links": {
    "self": { "href": "/api/my-bucket" },
    "up": { "href": "/api" }
  },
  "_embedded": {
    "directories": [
      {
        "name": "my-backups",
        "_links": { "self": { "href": "/api/my-bucket/my-backups" } }
      }
    ]
  }
}
```

Errors are still reported as plain text with the matching HTTP status code.

### Dependencies
During build time, *backmon* requires Go 1.26 or later.

### Development
CI runs [golangci-lint](https://golangci-lint.run) (including `gofmt`/`goimports`) on every pull request. To catch issues before pushing, install [pre-commit](https://pre-commit.com) and enable the hooks once per clone:

```bash
sudo dnf install pre-commit   # or: pipx install pre-commit
make hooks                    # runs `pre-commit install`
```

`make fmt` formats the code, `make lint` runs the linters, `make test` runs the tests.

### Helm
We provide a Helm chart for *backmon* which you can easily install:

```
$ helm repo add dreitier https://dreitier.github.io/helm-charts/
$ helm repo update
$ helm install backmon dreitier/backmon
```

The chart is also published as OCI artifact to the GitHub Container Registry; no `helm repo add` is needed (Helm 3.8+):

```
$ helm install backmon oci://ghcr.io/dreitier/charts/backmon
```

By default, the chart pulls the image from Docker Hub; set `image.repository=ghcr.io/dreitier/backmon` to use the GitHub Container Registry instead.

### Local installation

	$ mkdir -p ~/go/{src,bin}
	$ echo "export GOPATH=\"\$HOME/go\"" >> ~/.bashrc
	$ echo "export PATH=\"\$HOME/go/bin:\$PATH\"" >> ~/.bashrc
	$ curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

	# Ubuntu: Install latest Go version
	$ sudo add-apt-repository ppa:longsleep/golang-backports
	$ sudo apt update
	$ sudo apt install golang-go
	$ sudo go build

### Binaries
Binaries for Linux, macOS and Windows (amd64 and arm64) are attached to each [GitHub release](https://github.com/dreitier/backmon/releases), together with `checksums.txt` and SBOMs.

### Docker container
Multi-arch images (`linux/amd64`, `linux/arm64`) are published to two registries with the same content and tags:

| Registry                                                                                 | Image                      |
|------------------------------------------------------------------------------------------|----------------------------|
| [Docker Hub](https://hub.docker.com/r/dreitier/backmon)                                  | `dreitier/backmon`         |
| [GitHub Container Registry](https://github.com/dreitier/backmon/pkgs/container/backmon)  | `ghcr.io/dreitier/backmon` |

```bash
docker pull dreitier/backmon:5
docker pull ghcr.io/dreitier/backmon:5
```

Tags: `x.y.z`, `x.y`, `x` and `latest`. Mount your configuration to `/etc/backmon/config.yaml`.

The image is based on `distroless/static` and runs as the non-root user `65532`. If backmon has to read backup files that are only readable by root or a specific group, set the user or `fsGroup` accordingly (e.g. `--user` for `docker run`, `securityContext` in Kubernetes).

### Verifying releases
Releases are signed keyless with [Sigstore](https://www.sigstore.dev/): the signatures are bound to the identity of the release workflow in this repository, no keys are involved.

**Binaries and SBOMs** – `checksums.txt` is signed (`checksums.txt.sigstore.json` in the release) and lists the SHA-256 of every archive and SBOM:

```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/dreitier/backmon/.github/workflows/release.yml@refs/heads/main" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --ignore-missing -c checksums.txt
```

**Docker images** (Docker Hub and GHCR):

```bash
cosign verify ghcr.io/dreitier/backmon:5 \
  --certificate-identity "https://github.com/dreitier/backmon/.github/workflows/release.yml@refs/heads/main" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

In addition, binaries, SBOMs and images come with [build provenance attestations](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations) (SLSA), which also show the commit and workflow run they were built from:

```bash
gh attestation verify backmon_5.0.1_linux_amd64.tar.gz --owner dreitier
gh attestation verify oci://ghcr.io/dreitier/backmon:5.0.1 --owner dreitier
```

## Development
### Creating new releases
Releases are fully automated with [semantic-release](https://github.com/semantic-release/semantic-release). Every push to `main` runs the [release workflow](.github/workflows/release.yml): after CI passes, the next version is derived from the [Conventional Commits](https://www.conventionalcommits.org/) since the last release, `CHANGELOG.md` is updated, and the GitHub release with binaries and the Docker images are published.

| Commit / PR title                                  | Release                |
|----------------------------------------------------|------------------------|
| `fix: ...`, `perf: ...`, `build(deps): ...`        | patch (`4.0.0` → `4.0.1`) |
| `feat: ...`                                        | minor (`4.0.0` → `4.1.0`) |
| `feat!: ...` or a `BREAKING CHANGE:` footer        | major (`4.0.0` → `5.0.0`) |
| `docs:`, `ci:`, `chore:`, `refactor:`, `test:` ... | no release             |

PRs are squash-merged, so the **PR title** becomes the commit message on `main`; a check on every PR makes sure it follows the Conventional Commits format. Don't edit `CHANGELOG.md` or create tags by hand.

To try the GoReleaser build locally, run `make snapshot`.

### Running tests

```bash
make test       # unit tests; S3 integration tests are skipped, GCS tests use an in-memory fake
make test-s3    # S3 integration tests against S3Mock (requires Docker)
make test-gcs   # GCS integration tests against fake-gcs-server (requires Docker)
```

The S3 integration tests in `storage/provider/s3_integration_test.go` run against any S3-compatible server given in `BACKMON_TEST_S3_ENDPOINT` (e.g. `http://localhost:9090`). The Google Cloud Storage tests in `storage/provider/gcs_integration_test.go` run against the server given in `BACKMON_TEST_GCS_ENDPOINT` (e.g. `http://localhost:4443`), which has to implement the Cloud Storage JSON API like [fake-gcs-server](https://github.com/fsouza/fake-gcs-server); without it, they use an in-memory fake. CI runs all tests and comments on each pull request with its test coverage compared to `main`.

### Base images

The Dockerfile's base images are pinned by digest; Dependabot keeps tags and digests up to date. To refresh the digests by hand, run `make pin-images` (requires Docker).

## Changelog
The changelog is kept in the [CHANGELOG.md](CHANGELOG.md) file.

## Support
This software is provided as-is. You can open an issue in GitHub's issue tracker at any time. But we can't promise to get it fixed in the near future.
If you need professionally support, consulting or a dedicated feature, please get in contact with us through our [website](https://dreitier.com).

## Contribution
Feel free to provide a pull request.

## TODO
Please take a look in our [issue tracker](https://github.com/dreitier/backmon/issues).

## License
This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.
