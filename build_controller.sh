#!/bin/bash

declare tag="$1"

docker build -t splicemachine/kubernetes-ingress:${tag} -f build/Dockerfile .
