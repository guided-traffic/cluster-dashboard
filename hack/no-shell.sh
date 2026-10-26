#!/usr/bin/env bash
# Proves that each image given is distroless as docs/adr/0001 D3 requires: no shell starts, and
# no shell or busybox is among its files. `make docker-no-shell` runs it after `make docker-build`;
# the container scan of CI runs it on the image it scans.
#
#   hack/no-shell.sh IMAGE...
set -euo pipefail

[ "$#" -gt 0 ] || { echo "usage: $0 IMAGE..." >&2; exit 2; }

# The file names a shell lives under in the images this check guards against. dash, the Debian
# Almquist shell, counts under a bin directory only: the product's own binary is /dash.
shells='^(usr/)?(s)?bin/(sh|ash|bash|dash|zsh|busybox)$|^busybox/'

for image in "$@"; do
	docker image inspect "$image" >/dev/null 2>&1 || { echo "no image $image: build it first (make docker-build)" >&2; exit 1; }

	# 1. No shell starts: the runtime fails before anything runs, because the file is not there.
	if out=$(docker run --rm --entrypoint /bin/sh "$image" -c 'echo shell' 2>&1); then
		echo "FAIL: $image started /bin/sh: $out" >&2
		exit 1
	fi
	case "$out" in
	*"no such file or directory"* | *"not found"*) ;;
	*)
		echo "FAIL: $image: /bin/sh did not start, but not because it is missing: $out" >&2
		exit 1
		;;
	esac

	# 2. None is among its files: the image's file system, listed without running it.
	container=$(docker create "$image")
	listing=$(docker export "$container" | tar -t)
	docker rm "$container" >/dev/null
	if found=$(printf '%s\n' "$listing" | grep -E "$shells"); then
		echo "FAIL: $image holds a shell: $found" >&2
		exit 1
	fi
	echo "ok: $image has no shell (/bin/sh does not start; no sh, ash, bash, dash, zsh or busybox among its $(printf '%s\n' "$listing" | wc -l | tr -d ' ') files)"
done
