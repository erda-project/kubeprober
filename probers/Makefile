# default project
project = kubeprober
# version tag, default is latest if no input
V ?= latest

docker-build: ## build docker image.
	DOCKER_BUILDKIT=1 docker build --build-arg PROBER=${PROBER} --build-arg APPS=${APPS} -t ${project}/probe-${PROBER}:${V} -f Dockerfile ../

docker-push: ## push docker image.
	docker push ${project}/probe-${PROBER}:${V}

docker-build-push: docker-build docker-push
