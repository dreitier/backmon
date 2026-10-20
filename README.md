# backmon - Monitoring of backup files in filesystems and object storages
*backmon* monitors backup files inside a filesystem, or an S3-compatible object storage like AWS S3 or MinIO.

## Description

With *backmon* you can monitor and check the presence, size and timestamps of your recurring backup files. Your backup files can be stored either in a local filesystem or inside an S3-compatible object storage like AWS S3 or MinIO.

You can easily integrate *backmon* into your Prometheus- and Grafana-based infrastructure for analysing the duration of creating backups or alerting if a backup fails some constraints.

## Getting started
### Documentation
You can find our official documentation at [https://dreitier.github.io/backmon-docs](https://dreitier.github.io/backmon-docs).

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
$ helm install dreitier/backmon
```

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
Multi-arch images (`linux/amd64`, `linux/arm64`) are published to [Docker Hub](https://hub.docker.com/r/dreitier/backmon) and GHCR:

```bash
docker pull dreitier/backmon:4        # or ghcr.io/dreitier/backmon:4
```

Tags: `x.y.z`, `x.y`, `x` and `latest`. Mount your configuration to `/etc/backmon/config.yaml`.

The image is based on `distroless/static` and runs as the non-root user `65532`. If backmon has to read backup files that are only readable by root or a specific group, set the user or `fsGroup` accordingly (e.g. `--user` for `docker run`, `securityContext` in Kubernetes).

### Verifying releases
Binaries and images come with [build provenance attestations](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations):

```bash
gh attestation verify backmon_4.0.0_linux_amd64.tar.gz --owner dreitier
gh attestation verify oci://ghcr.io/dreitier/backmon:4.0.0 --owner dreitier
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
make test
```

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
