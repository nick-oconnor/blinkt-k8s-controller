#!/bin/bash

# Create a bare Docker image with just the Controller binary
# This assumes a working Docker installation

set -xe

docker build -t ngpitt/blinkt-k8s-controller-nodes:v1 .
docker push ngpitt/blinkt-k8s-controller-nodes
