# Vistecture Dashboard

Docker: `aoepeople/vistecture-dashboard`

Works together with [Vistecture](https://github.com/aoepeople/vistecture) and shows the state of the vistecture architecture in kubernetes.

Development:

```
# install dep
go get -u github.com/golang/dep/cmd/dep

# get dependencies
dep ensure

# run
go run vistecture-dashboard.go k8s.go
```

Run: `vistecture-dashboard`

Example Project Dockerfile

```dockerfile
FROM aoepeople/vistecture-dashboard

COPY definition /go/src/github.com/AOEpeople/project

EXPOSE 8080

CMD ["vistecture-dashboard"]

WORKDIR /go/src/github.com/AOEpeople/vistecture-dashboard/
```

Vistecture Properties

- `healthcheck`: Healthcheck endpoint
- `deployment`: Has to be set to `kubernetes`
- `kubernetes-name`: Override name
