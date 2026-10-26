#!/usr/bin/env bash
# The envtest binaries (docs/adr/0003 D2; docs/developer/testing.md): etcd and kube-apiserver for
# the backend's integration tests, tagged integration. `make envtest-bin` and `make
# test-integration` run it. Taken from Cogpit's hack/envtest/fetch.sh, unchanged in what it does.
#
#   hack/envtest/fetch.sh <kubernetes version> <platform> <sha512> <dir>
#
# It downloads envtest-v<version>-<platform>.tar.gz from controller-tools' release store (the
# archives setup-envtest and controller-runtime's envtest download, listed with their SHA-512 in
# https://raw.githubusercontent.com/kubernetes-sigs/controller-tools/HEAD/envtest-releases.yaml),
# refuses it unless its SHA-512 is the pinned one (the Makefile's ENVTEST_SHA512_<platform>), and
# keeps etcd and kube-apiserver in <dir> (.cache/envtest/<version>). kubectl, also in the archive,
# is left out: no test runs it. Nothing is installed anywhere else.
set -euo pipefail

die() {
	if [ -n "${GITHUB_ACTIONS:-}" ]; then printf '::error::%s\n' "$*" >&2; else printf 'error: %s\n' "$*" >&2; fi
	exit 1
}

sha512() {
	if command -v sha512sum >/dev/null; then sha512sum "$1" | cut -d' ' -f1; else shasum -a 512 "$1" | cut -d' ' -f1; fi
}

[ "$#" -eq 4 ] || die "usage: $0 <kubernetes version> <platform> <sha512> <dir>"
version=$1 platform=$2 sha=$3 dir=$4
case "$version" in "" | *[!0-9.]*) die "Kubernetes version $version: digits and dots" ;; esac
[ -n "$platform" ] || die "no envtest archive pinned for this platform: envtest is pinned for darwin-arm64 and linux-amd64 (the Makefile's ENVTEST_PLATFORM_* and ENVTEST_SHA512_*)"
case "$platform" in *[!a-z0-9-]*) die "envtest platform $platform: <os>-<arch>" ;; esac
[ -n "$sha" ] || die "no pinned SHA-512 for $platform"
case "$sha" in *[!0-9a-f]*) die "the pinned SHA-512 is not lower-case hex" ;; esac
[ "${#sha}" = 128 ] || die "the pinned SHA-512 has ${#sha} characters, not 128"
name="envtest-v$version-$platform"
url="https://github.com/kubernetes-sigs/controller-tools/releases/download/envtest-v$version/$name.tar.gz"
mkdir -p "$(dirname "$dir")"
# Global, for the trap that removes it when the script ends.
tmp="$(mktemp -d "$(dirname "$dir")/.envtest.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 -o "$tmp/envtest.tar.gz" "$url"
got="$(sha512 "$tmp/envtest.tar.gz")"
[ "$got" = "$sha" ] || die "$name.tar.gz has SHA-512 $got, the pin is $sha"
mkdir "$tmp/x"
tar -xzf "$tmp/envtest.tar.gz" -C "$tmp/x" controller-tools/envtest/etcd controller-tools/envtest/kube-apiserver
for bin in etcd kube-apiserver; do
	[ -f "$tmp/x/controller-tools/envtest/$bin" ] || die "the archive holds no $bin"
	chmod 0755 "$tmp/x/controller-tools/envtest/$bin"
done
printf '%s  %s.tar.gz\n' "$sha" "$name" >"$tmp/x/controller-tools/envtest/.verified"
rm -rf "$dir"
mv "$tmp/x/controller-tools/envtest" "$dir"
echo "envtest $version ($platform) in $dir: $("$dir/kube-apiserver" --version), etcd $("$dir/etcd" --version | sed -n 's/^etcd Version: //p')"
