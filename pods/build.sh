#!/bin/bash

# Build a statically linked Linux executable to be used in the Docker container
# This assumes a properly configured Go installation

set -xe

GOOS=linux GOARCH=arm go build -v
