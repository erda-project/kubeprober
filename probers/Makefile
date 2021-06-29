# default project
project = kubeprober
# version tag, default is latest if no input
V ?= latest

docker-build: ## build docker image.
	docker build --build-arg APP=${APP} -t ${project}/${APP}:${V} -f Dockerfile ../

docker-push: ## push docker image.
	docker push ${project}/${APP}:${V}

docker-build-push: docker-build docker-push
