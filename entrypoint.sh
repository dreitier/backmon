#!/usr/bin/env sh

if [ -f /etc/cloudmon/config-raw.yaml ]; then
	echo "Configuration file template '/etc/cloudmon/config-raw.yaml' exiting, replacing placeholders by environment variables"
	interpolator /etc/cloudmon/config-raw.yaml /etc/cloudmon/config.yaml
fi

/usr/local/bin/cloudmon "$@"
