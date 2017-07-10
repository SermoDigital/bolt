#!/usr/bin/env bash

set -euo pipefail

readonly lpath=src/github.com/sermodigital/bolt
rsync -av --progress --exclude _testdata/ "$GOPATH/${lpath}" . 

readonly name="bolt_build_test"
function test_build {
cat << EOF > Dockerfile
FROM golang:1.${1}
ADD $(basename "${lpath}") "${lpath}"
RUN cd "${lpath}" && go build
EOF

docker build -t "${name}" .
docker run "${name}"
}

declare -a vers=("7" "8" "9")
for v in "${vers[@]}"
do
	test_build "${v}"
done
