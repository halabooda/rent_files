#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

. /usr/local/share/load-env.sh

/usr/local/bin/ffmpeg -h

exec tusd "$@"
