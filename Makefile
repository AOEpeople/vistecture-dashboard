VERSION=2.1.3

.PHONY: docker dockerpublish test

default: test

test:
	go test -vet=all ./...

docker:
	docker build -t aoepeople/vistecture-dashboard .

dockerpublish: docker
	docker tag aoepeople/vistecture-dashboard:latest aoepeople/vistecture-dashboard:$(VERSION)
	docker push aoepeople/vistecture-dashboard:latest
	docker push aoepeople/vistecture-dashboard:$(VERSION)
