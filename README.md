# cloudmon - A tool for monitoring and downloading backups
`cloudmon` scans configured S3 or S3-compatible (like Minio) buckets and optionally returns tha latest file for downloading..

# Dependencies
Cloudmon requires Go 1.11 or later to make use of Go modules (@see https://medium.com/mindorks/create-projects-independent-of-gopath-using-go-modules-802260cdfb51)

# Setup

	$ mkdir -p ~/go/{src,bin}
	$ echo "export GOPATH=\"\$HOME/go\"" >> ~/.bashrc
	$ echo "export PATH=\"\$HOME/go/bin:\$PATH\"" >> ~/.bashrc
	$ curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
	
	# Ubuntu: Install latest Go version
	$ sudo add-apt-repository ppa:longsleep/golang-backports
	$ sudo apt update
	$ sudo apt install golang-go
	$ sudo go build

# Configuration
`cloudmon` tries to locate the configuration file `config.yaml` below the following directories (priority in the defined order):
- local directory
- `${HOME}/.cloudmon`
- `/etc/cloudmon`

In the configuration file, you can use placeholders like `${VAR}`. Those placeholders will be replaced during the startup of cloudmon with the corresponding environment variables. You have to place the configuration file at `/etc/cloudmon/config-raw.yaml`.

## Fields

### purge
If *purge* is set to `true`, without having an explicit `retention`, a limit von 14 is assumed.

### sort
If no sorting behaviour is defined, the sorting algorithm `hybrid` is used. `hybrid` uses substituion-based date fields and fills all missing information with help of `mtime` of the file.

## New fields

### alias
Can be used on a `directory` and a `file`. The alias is used when exporting metrics through the HTTP API.

### fuse
`fuse` is the replacement for the original `grouping` field. In contract, `fuse` has to be defined on a per-directory level.

## Variables
A variable can be put on a *directory* pattern by using two curly braces (e.g. `{{variable}}`). For variable names only characters `0-9`, `A-Z`, `a-z`, and `_` are allowed.
Variables can be referenced in a *file* pattern with help of `${variable}`.

## Substitutions
A substitution consists upon two characters, beginning with a percent sign `%`.
They can be used on a *directory* and *file* pattern, to restrict the search to specific patterns.

### Usable substitutions and their equivalent regular epxressions

| Substitution | Description                            | Regex                      |
| ------------ | -------------------------------------- | -------------------------- |
| %Y           | Year, 4 characters                     | `[0-9]{4}`                 |
| %y           | Year, 2 characters                     | `[0-9]{2}`                 |
| %M           | Month, 2 characte                      | `0[1-9]|1[0-2]`            |
| %D           | Day, 2 characters                      | `0[1-9]|[1,2][0-9]|3[0,1]` |
| %h           | Hour, 2 characters                     | `[0,1][0-9]|2[0-3]`        |
| %m           | Minute, 2 characters                   | `[0-5][0-9]`               |
| %s           | Second, 2 characters                   | `[0-5][0-9]`               |
| %i           | Decimal number, without leading `0`    | `0|[1-9][0-9]*`            |
| %I           | Decimal number, with leading `0`       | `[0-9]+`                   |
| %x           | Hex number in lower case               | `[0-9a-f]+`                |
| %X           | Hex number in upper case               | `[0-9A-F]+`                |
| %w           | One or multiple words                  | `/w+`                      |
| %v           | Same regex as for variables            | `[^\\./]+`                 |
| %?           | One or multiple characters             | `.+`                       |

## Missing features
The following fields should be available in cloudmon but are not usable yet:
- `cron`
- `sort`
- `purge` (only for testing purposes, see `storage.go`, around line 200)
