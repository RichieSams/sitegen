#!/bin/sh
set -e

if [ "$1" = "build" ]; then
	shift
	exec /usr/local/bin/sitegen build "$@"
elif [ "$1" = "serve" ]; then
	shift
	exec /usr/local/bin/sitegen serve "$@"
fi

exec "$@"
