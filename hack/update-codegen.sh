#!/usr/bin/env bash

# Copyright 2024 Open Defense Cloud Authors.
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

set -o errexit
set -o nounset
set -o pipefail

# Get the directory of this script
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${SCRIPT_ROOT}"

# Module and paths
MODULE="github.com/opendefensecloud/solution-arsenal"
API_PKG="${MODULE}/pkg/apis"
OUTPUT_PKG="${MODULE}/pkg/client"
GROUP_VERSION="solar:v1alpha1"

echo "Generating deepcopy functions..."
go run sigs.k8s.io/controller-tools/cmd/controller-gen \
    object:headerFile="hack/boilerplate.go.txt" \
    paths="./pkg/apis/..."

echo "Generating clientset, listers, and informers..."

# Generate clientset
go run k8s.io/code-generator/cmd/client-gen \
    --clientset-name "versioned" \
    --input-base "" \
    --input "${API_PKG}/solar/v1alpha1" \
    --output-pkg "${OUTPUT_PKG}/clientset" \
    --output-dir "pkg/client/clientset" \
    --go-header-file "hack/boilerplate.go.txt"

# Generate listers
go run k8s.io/code-generator/cmd/lister-gen \
    --input-dirs "${API_PKG}/solar/v1alpha1" \
    --output-pkg "${OUTPUT_PKG}/listers" \
    --output-dir "pkg/client/listers" \
    --go-header-file "hack/boilerplate.go.txt"

# Generate informers
go run k8s.io/code-generator/cmd/informer-gen \
    --input-dirs "${API_PKG}/solar/v1alpha1" \
    --versioned-clientset-package "${OUTPUT_PKG}/clientset/versioned" \
    --listers-package "${OUTPUT_PKG}/listers" \
    --output-pkg "${OUTPUT_PKG}/informers" \
    --output-dir "pkg/client/informers" \
    --go-header-file "hack/boilerplate.go.txt"

echo "Code generation complete!"
