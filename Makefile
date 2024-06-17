VERSION=2.3.1

.PHONY: docker dockerpublish test run run-demo

default: test

run:
	go run vistecture-dashboard.go

run-demo:
	go run vistecture-dashboard.go -config=example/project.yml -Demo

test:
	go test -vet=all ./...

docker:
	docker buildx build --tag aoepeople/vistecture-dashboard:latest --platform linux/amd64 .

dockerpublish:
	docker buildx build \
    				--push \
    				--tag aoepeople/vistecture-dashboard:latest \
    				--tag aoepeople/vistecture-dashboard:$(VERSION) \
    				--platform linux/amd64 .