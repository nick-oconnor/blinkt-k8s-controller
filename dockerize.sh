# Create a bare Docker image with just the Controller binary
# This assumes a working Docker installation

docker build -t apprenda/blinkt-k8s-controller:v1 .
docker push apprenda/blinkt-k8s-controller:v1