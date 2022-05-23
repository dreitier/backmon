# cloudmon - Monitoring of backups in S3 buckets
*cloudmon* monitors backup file inside an S3 or Minio bucket.

# Dependencies
*cloudmon* requires Go 1.11 or later to make use of Go modules (@see https://medium.com/mindorks/create-projects-independent-of-gopath-using-go-modules-802260cdfb51)

# Local installation

	$ mkdir -p ~/go/{src,bin}
	$ echo "export GOPATH=\"\$HOME/go\"" >> ~/.bashrc
	$ echo "export PATH=\"\$HOME/go/bin:\$PATH\"" >> ~/.bashrc
	$ curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
	
	# Ubuntu: Install latest Go version
	$ sudo add-apt-repository ppa:longsleep/golang-backports
	$ sudo apt update
	$ sudo apt install golang-go
	$ sudo go build

# License
TBD

# Documentation
You can find the official documentation at [https://cloudmon.github.io](https://cloudmon.github.io).
