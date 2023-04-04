#!/bin/bash -e
# Copyright 2023 Google LLC
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

# ========================================================================

# This script calls protoc to generate Go client code for the gRPC service
# defined by api/v3alpha/api.proto.
#
# For information about generating client code for other languages and
# platforms, please see https://grpc.io/docs/

cd $(dirname "$0")/../../../../../../

protoc \
  --go_out=examples/go/package_lock_licenses/internal \
  --go-grpc_out=paths=source_relative:examples/go/package_lock_licenses/internal \
  --proto_path=. \
  api/v3alpha/api.proto
