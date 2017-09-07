#!/bin/bash

# Build a statically linked Linux executable to be used in the Docker container
# This assumes a properly configured Go installation

set -xe

cd pods
./build.sh
cd ../nodes
./build.sh
cd ..
