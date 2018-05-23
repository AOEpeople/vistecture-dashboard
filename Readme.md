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
go run vistecture-dashboard.go
```

For a demo display please use:
```
go run vistecture-dashboard.go -config=example/vistecture-config.yml -Demo=1
```


And access it via http://localhost:8080

Run: `vistecture-dashboard`

Example Project Dockerfile

```dockerfile
FROM aoepeople/vistecture-dashboard

COPY definition /go/src/github.com/AOEpeople/project

EXPOSE 8080

CMD ["vistecture-dashboard"]

WORKDIR /go/src/github.com/AOEpeople/vistecture-dashboard/
```

Vistecture Properties that are used:

- `healthCheckPath`: Healthcheck endpoint (relative path) (Optional - if not set just the base url is called) - If a healthCheckPath is configured it need to match the defined format (see below)
- `apiDocPath`: Optional the relative path to an API spec (just used to show a link)
- `deployment`: Has to be set to `kubernetes` (otherwise app is not checked)
- `k8sDeploymentName`: Override the name of the deployment in kubernetes that is checked(default = appname)
- `k8sHealthCheckServiceName`: Override service name that is used to check health (default = appname)
- `k8sHealthCheckThroughIngress`: If the app should be checked from public (ingress is required for the service)
- `k8sType`: set to "job" if the application is not represented by an deployment in kubernetes, but it is just a job

## Healtcheck Format:

```

```