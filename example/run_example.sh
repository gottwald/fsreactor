#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

fsreactor -c config.cfg &

function cleanup {
    kill %1
}
trap cleanup EXIT

sleep 1

echo "change to file" > testfile

sleep 1

cat logfile.txt
