# Create a bare Docker image with just the Controller binary
# This assumes a working Docker installation

docker build -t apprenda/blinkt-k8s-controller:v2 .
docker push apprenda/blinkt-k8s-controller:v2
