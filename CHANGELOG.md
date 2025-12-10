# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [3.2.2] - 2025-12-10
### Fixed
- missing indirection in test
- failing test when parsing YAML
- fix memory leak when streaming large S3 files to the client

### Added
- provide content length header when downloading a file

## [3.2.1] - 2025-12-02

### Fixed
- fixed downloading a file from the root of the bucket

## [3.2.0] - 2025-11-20

### Added
- in-cluster config via IRSA
- assume role functionality

### Fixed
- fixed typo in storage.go

## [3.1.0] - 2025-05-15

### Added

- Templates are now supported in the config file. For example, the following is now possible:
```yaml
endpoint: "__${S3_ENDPOINT_HOST}__:__${S3_ENDPOINT_PORT}__"
```

## [3.0.0] - 2025-03-14

### Changed

- BREAKING: Changed the layout of the `backup_definitions.yaml` file. The list of directory definitions is no longer a
  top level element but has moved below `directories`. You __have__ to update your definitions files accordingly.
- BREAKING: The S3 specific properties like endpoint, credentials etc. have moved from directly below the environment to
  the key `s3`. For example:

```yaml
environments:
  my-aws-env:
    s3:
      access_key_id: my-access-key
      secret_access_key: my-secret-access-key
      auto_discover_disks: true
```

You __MUST__ update your config files accordingly!

- BREAKING: The following metrics have been renamed

| old                                        | new                                                               | comment |
|--------------------------------------------|-------------------------------------------------------------------|---------|
| backmon_backup_file_count_aim              | backmon_backup_file_count_max                                     |         |
| backmon_backup_file_age_aim_seconds        | backmon_backup_file_age_max_seconds                               |         |
| backmon_backup_latest_creation_aim_seconds | backmon_backup_latest_file_creation_expected_at_timestamp_seconds |         |
| backmon_backup_latest_creation_seconds     | backmon_backup_latest_file_created_at_timestamp_seconds           |         |

### ADDED:

- `backmon_file_count_total` - reports the total amount of objects on a disk
- `backmon_disk_usage_bytes` - reports the total amount of space used by all object on a disk
- `backmon_disk_quota_bytes` - reports the quota on a disk. This requires the respective quota to be configured in the
  definitions file. For example

```yaml
quota: 24GiB
```

will result in the following metric to be reported:

```text
backmon_disk_quota_bytes{disk="my-disk"} 2.5769803776e+10
```

## [2.2.0] - 2024-01-09

### Changed

- BREAKING: Changed global config file section `disks` to be environment specific (#29). You __have__ to update your
  configuration file accordingly.

## [2.0.0] - 2022-08-24

### Changed

- BREAKING: Renamed product name from `cloudmon` to `backmon` (#14). You __have__ to change the files `.cloudmonignore`
  to `.backmonignore` and `/etc/cloudmon/cloudmon.yaml` to `/etc/backmon/backmon.yaml`.

### Added

- Received SIGHUP reloads disk configuration (#10)
- Support for TLS encryption (#11)

### Fixed

- IAM permission ListAllMyBuckets is no longer needed (#9)

## [1.5.1] - 2022-08-03

### Added

- Log output will now show date and time
- *cloudmon* supports `-background` parameter to disable interactive terminal

### Fixed

- *cloudmon* does not exit when running without any `/dev/tty`

## [1.5.0] - 2022-08-02

### Added

- BREAKING: Add support for fine grained include/exclude definitions AKA white/gray/blacklisting (#5). Please note that
  the old `ignore_disks:` section is no longer available. You have to move those configuration values into the
  `disks.exclude:` section
- Disks can be included and excluded with regular expressions (#6)
- Files can now be sorted by `born_at`, `modified_at`, `archived_at` and `interpolation`. Configuration parameters
  `defaults.sort` and `files.*.sort` are activated.
- Disks can be refreshed by hitting `Ctrl+R` or just `r` in the console

### Fixed

- `-debug` had no effect when log setting in `config.yaml` was missing

## [1.4.0] - 2022-07-19

### Added

- Support for `.stat` backup files to trace a backup file's born date, modification date and upload date.
- New Prometheus exports for `.stat` metrics: `latest_file_born_at`, `latest_file_modified_at`,
  `latest_file_archived_at` and `latest_file_creation_duration`

### Removed

- Unused code for fetching files from S3
- Duplicate type _File_ as it has also not been used

## [1.3.1] - 2022-07-12

### Fixed

- updated README.md to reflect the naming change from _ignore_buckets_ to _ignore_disks_
- don't try to list items in ignored disks to avoid errors

## [1.3.0] - 2022-07-05

### Added

- Version (Git tag) and Git commit is shown during startup

## [1.2.1] - 2022-06-29

### Added

- `http.basic_auth` can now be used,
  see [configuration overview](https://dreitier.github.io/cloudmon-docs/reference/cloudmon-configuration/overview).

## [1.2.0] - 2022-05-30

### Changed

- Replaced `bucket` with `disk` to make a clear distinction between S3 *buckets* and *cloudmon* *disks*

### Fixed

- Files at root level now get detected properly
- Valid aliases no longer get reported as invalid

## [1.0.0] - 2019-12-04

### Added

- `retention-age` setting, to purge files based on their age
- New metric `files_maturity_seconds`, which reports the value of `retention-age`
- New metric `files_young`, which reports the number of files that are younger than the retention age
- New metric `latest_creation_aim_seconds`, which reports the last time at which a backup should have occurred - based
  on the schedule specified in `cron`
- Global config option `ignore_disks`, which allows for ignoring buckets by name
- New metric `status`, which reports values `≥1` if there were errors while scraping a bucket, and `0` if the bucket is
  ok

### Changed

- Renamed the setting `retention` to `retention-count`
- Purging now only deletes files that exceed both `retention-count` and `retention-age`
- Moved `cron` setting from definition level into file level
- Renamed the `cron` setting to `schedule`
- You can now have only one backup definition per bucket, to reduce complexity:
  - Directories are now at top level in `backup_definitions.yaml`
  - Metrics no longer have the label `definition`
  - The web api now navigates directly from bucket to directory
- The config option `update_interval` now expects a duration expression
- Renamed the following metrics:

| old name                 | new name               |
|--------------------------|------------------------|
| `files_expected`         | `file_count_aim`       |
| `files_present`          | `file_count`           |
| `files_maturity_seconds` | `file_age_aim_seconds` |
| `files_young`            | `file_young_count`     |

### Removed

- Removed the setting `size`, since it was unused
- Removed the metric `extraneous_files`, since it was unused
- Removed the metric `definition_exists`, in favor of the new metric `status`

### Fixed

- Trying to access nonexistent files through the web api no longer crashes the server
- We no longer warn if an alias contains spaces, as most browsers handle them fine

## [0.10.1] - 2019-09-27

### Fixed

- Enabled purging code
- Fixed oversight in s3 Delete code that caused objects to not get deleted properly

## [0.10.0] - 2019-09-12

### Added

- Added interpolator to the Docker image, so variable placeholders in the config file can now be replaced by environment
  variables

## [0.9.0] - 2019-09-11

initial release

### Added
