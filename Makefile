VERSION=0.2

dockerpublish:
	docker build --no-cache -t aoepeople/vistecture-dashboard .
	docker tag aoepeople/vistecture-dashboard:latest aoepeople/vistecture-dashboard:$(VERSION)
	docker push aoepeople/vistecture-dashboard