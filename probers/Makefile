# default project
project = kubeprober
# version tag, default is latest if no input
V ?= latest

docker-build: ## build docker image.
	docker build --build-arg PROBER=${PROBER} --build-arg APPS=${APPS} -t ${project}/${PROBER}:${V} -f Dockerfile ../

docker-push: ## push docker image.
	docker push ${project}/${PROBER}:${V}

docker-build-push: docker-build docker-push
