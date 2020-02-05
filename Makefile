NAME = splicemachine/kubernetes-ingress
VERSION = 2.0.11_0.0.1

.PHONY: all build run ssh stop clean push realclean

all: build

build:
	docker build -t $(NAME):$(VERSION) -f build/Dockerfile .

push:
	docker push $(NAME):$(VERSION)

clean:
	-docker rmi $(NAME):$(VERSION)
