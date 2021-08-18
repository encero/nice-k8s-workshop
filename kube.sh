#!/bin/sh

docker run --rm -it --network host -v $(pwd):/workshop kube
