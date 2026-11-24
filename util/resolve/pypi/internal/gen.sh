#!/bin/bash -e
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script runs the official Python Docker container to regenerate env.gen.go
# (the PEP 425 platform tags and PEP 508 environment markers used for the
# resolver). Expects one argument: the tag for the python Docker image (e.g.
# "3.14").

if [ -z "$1" ]; then
  echo "Python version tag must be specified (e.g. ./gen.sh 3.14)" >&2
  exit 1
fi

cd "$(dirname "$0")"

image="python:$1"
echo "Using image: $image"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

docker run --rm -v "$(pwd):/workspace" -w /workspace "$image" python3 gen.py > "$tmp"
mv "$tmp" env.gen.go
gofmt -w -s env.gen.go
