# cloudmon - Monitoring of backup files in filesystems and object storages
*cloudmon* monitors backup files inside a filesystem, or an S3-compatible object storage like AWS S3 or MinIO.

## Description

With *cloudmon* you can monitor and check the presence, size and timestamps of your recurring backup files. Your backup files can be stored either in a local filesystem or inside an S3-compatible object storage like AWS S3 or MinIO.

You can easily integrate *cloudmon* into your Prometheus- and Grafana-based infrastructure for analysing the duration of creating backups or alerting if a backup fails some constraints.

## Getting started
### Documentation
You can find our official documentation at [https://dreitier.github.io/cloudmon-docs](https://dreitier.github.io/cloudmon-docs).

### Dependencies
*cloudmon* requires Go 1.11 or later to make use of Go modules, see [https://medium.com/mindorks/create-projects-independent-of-gopath-using-go-modules-802260cdfb51](https://medium.com/mindorks/create-projects-independent-of-gopath-using-go-modules-802260cdfb51).

### Helm

We provide a Helm chart for *cloudmon* which you can easily install:

```
$ helm repo add dreitier https://dreitier.github.io/helm-charts/
$ helm repo update
$ helm install dreitier/cloudmon
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

### Docker container
You can find ready-to-run Docker containers at [dreitier/cloudmon](https://hub.docker.com/repository/docker/dreitier/cloudmon).

## Development
### Creating new releases

1. Update the [CHANGELOG.md](changelog).
2. Create a new release (artifact & Docker container) by pushing a new Git tag:

```bash
$ git tag x.y.z
$ git push origin x.y.z
```

## Changelog
The changelog is kept in the [CHANGELOG.md](CHANGELOG.md) file.

## Support
This software is provided as-is. You can open an issue in GitHub's issue tracker at any time. But we can't promise to get it fixed in the near future.
If you need professionally support, consulting or a dedicated feature, please get in contact with us through our [website](https://dreitier.com).

## Contribution
Feel free to provide a pull request.

## TODO
Please take a look in our [issue tracker](https://github.com/dreitier/cloudmon/issues).

## License
This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.
