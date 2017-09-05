# Create a bare Docker image with just the Controller binary
# This assumes a working Docker installation

docker build -t ngpitt/blinkt-k8s-controller-pods -f pods-dockerfile .
docker push ngpitt/blinkt-k8s-controller-pods
docker build -t ngpitt/blinkt-k8s-controller-nodes -f nodes-dockerfile .
docker push ngpitt/blinkt-k8s-controller-nodes
