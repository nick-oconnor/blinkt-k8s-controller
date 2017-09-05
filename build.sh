# Build a statically linked Linux executable to be used in the Docker container
# This assumes a properly configured Go installation

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=arm

go build -a -installsuffix cgo pods.go
go build -a -installsuffix cgo nodes.go
