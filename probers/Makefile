

docker-build: ## Build docker image with the manager.
	version="latest"
	project="kubeprober"
	docker build --build-arg APP=${APP} -t ${project}/${APP}:${VERSION-$version} .

