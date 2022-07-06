# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## 1.3.0 - 2022-07-05
### Added
- Version (Git tag) and Git commit is shown during startup

## 1.2.1 - 2022-06-29
### Added
- `http.basic_auth` can now be used, see [configuration overview](https://dreitier.github.io/cloudmon-docs/reference/cloudmon-configuration/overview).

## 1.2.0 - 2022-05-30
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
- New metric `latest_creation_aim_seconds`, which reports the last time at which a backup should have occurred - based on the schedule specified in `cron`
- Global config option `ignore_buckets`, which allows for ignoring buckets by name
- New metric `status`, which reports values `≥1` if there were errors while scraping a bucket, and `0` if the bucket is ok

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
    | :----------------------- | :--------------------- |
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
- Added interpolator to the Docker image, so variable placeholders in the config file can now be replaced by environment variables

## [0.9.0] - 2019-09-11
initial release
### Added
