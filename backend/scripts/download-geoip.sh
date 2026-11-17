#!/bin/sh
# Downloads DB-IP's IP to Country Lite (CC BY 4.0, https://db-ip.com) to the
# path in $1 (ADR 0007). DB-IP publishes one file a month; early in a month
# the new one may not be out yet, so the previous month is the fallback.
# Runs on the busybox of the Docker build and on a regular Linux shell.
set -eu

out=${1:?usage: download-geoip.sh <output.mmdb>}
mkdir -p "$(dirname "$out")"

year=$(date -u +%Y)
month=$(date -u +%m | sed 's/^0//')
if [ "$month" -eq 1 ]; then
	previous=$(printf '%04d-%02d' $((year - 1)) 12)
else
	previous=$(printf '%04d-%02d' "$year" $((month - 1)))
fi
current=$(printf '%04d-%02d' "$year" "$month")

fetch() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1"
	else
		wget -qO- "$1"
	fi
}

for edition in "$current" "$previous"; do
	url="https://download.db-ip.com/free/dbip-country-lite-$edition.mmdb.gz"
	if fetch "$url" | gunzip > "$out.tmp" 2>/dev/null && [ -s "$out.tmp" ]; then
		mv "$out.tmp" "$out"
		echo "DB-IP country database $edition saved to $out"
		exit 0
	fi
done

rm -f "$out.tmp"
echo "could not download the DB-IP country database" >&2
exit 1
