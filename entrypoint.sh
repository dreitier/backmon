#!/usr/bin/env sh

if [ -f /etc/backmon/config-raw.yaml ]; then
	echo "Configuration file template '/etc/backmon/config-raw.yaml' exists, replacing placeholders by environment variables"
	$(pwd)/interpolator /etc/backmon/config-raw.yaml /etc/backmon/config.yaml
fi

$(pwd)/backmon "$@"
