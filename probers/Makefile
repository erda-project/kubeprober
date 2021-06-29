# default project
project = kubeprober
# version tag, default is latest if no input
V ?= latest

docker-build: ## Build docker image with the manager.
	docker build --build-arg APP=${APP} -t ${project}/${APP}:${V} .

