IMAGE=dreitier/cloudmon
TAG=1.0.0
EXECUTABLE=cloudmon

all: build docker-build

build:
	# disable CGO to use default libc
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(EXECUTABLE)
	strip $(EXECUTABLE)

docker-build:
	docker build --no-cache -t ${IMAGE}:${TAG} .

docker-push:
	docker tag ${IMAGE}:${TAG} ${IMAGE}:latest
	docker push ${IMAGE}:${TAG}
	docker push ${IMAGE}:latest