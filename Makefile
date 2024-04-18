VERSION=2.2.2

.PHONY: docker dockerpublish test run run-demo

default: test

run:
	go run vistecture-dashboard.go

run-demo:
	go run vistecture-dashboard.go -config=example/project.yml -Demo

test:
	go test -vet=all ./...

docker:
	docker build -t aoepeople/vistecture-dashboard .

dockerpublish: docker
	docker tag aoepeople/vistecture-dashboard:latest aoepeople/vistecture-dashboard:$(VERSION)
	docker push aoepeople/vistecture-dashboard:latest
	docker push aoepeople/vistecture-dashboard:$(VERSION)
