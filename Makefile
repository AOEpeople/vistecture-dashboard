VERSION=0.2.2

dockerpublish:
	echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin
	docker build --no-cache -t aoepeople/vistecture-dashboard .
	docker tag aoepeople/vistecture-dashboard:latest aoepeople/vistecture-dashboard:$(VERSION)
	docker push aoepeople/vistecture-dashboard
