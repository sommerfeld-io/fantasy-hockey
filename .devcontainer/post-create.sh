#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset
# set -o xtrace

echo "[INFO] Initialize pre-commit"
pre-commit install

task bmad:install
