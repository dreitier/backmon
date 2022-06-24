# cloudmon - Monitoring of backups in S3 buckets
*cloudmon* monitors backup file inside an S3 or Minio bucket.

## Description

With *cloudmon* you can monitor and check recurring backup files, stored in an S3-compatible bucket. *cloudmon* exports the backup status of the files in a Prometheus-compatible format.

## Getting started
### Documentation
The official documentation is located at [https://dreitier.github.io/cloudmon-docs](https://dreitier.github.io/cloudmon-docs).

### Dependencies
*cloudmon* requires Go 1.11 or later to make use of Go modules, see [https://medium.com/mindorks/create-projects-independent-of-gopath-using-go-modules-802260cdfb51](https://medium.com/mindorks/create-projects-independent-of-gopath-using-go-modules-802260cdfb51).

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
A new release (artifact & Docker container) is automatically created when a new Git tag is pushed:

```bash
git tag x.y.z
git push origin x.y.z
```

## Support
This software is provided as-is. You can open an issue in GitHub's issue tracker at any time. But we can't promise to get it fixed in the near future.
If you need professionally support, consulting or a dedicated feature, please get in contact with us through our [website](https://dreitier.com).

## Contribution
Feel free to provide a pull request.

## TOOD

- `default.sort` and `files[].sort` has no effect at the moment

## License
This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.
