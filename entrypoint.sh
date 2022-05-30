#!/usr/bin/env sh

if [ -f /etc/cloudmon/config-raw.yaml ]; then
	echo "Configuration file template '/etc/cloudmon/config-raw.yaml' exists, replacing placeholders by environment variables"
	$(pwd)/interpolator /etc/cloudmon/config-raw.yaml /etc/cloudmon/config.yaml
fi

$(pwd)/cloudmon "$@"
