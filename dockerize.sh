#!/bin/bash

# Create a bare Docker image with just the Controller binary
# This assumes a working Docker installation

set -xe

cd pods
./dockerize.sh
cd ../nodes
./dockerize.sh
cd ..
