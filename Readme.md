# Vistecture Dashboard

Docker: `aoepeople/vistecture-dashboard`

Works together with [Vistecture](https://github.com/aoepeople/vistecture) and shows the state of the vistecture architecture in kubernetes like this:

![Vistecture_Dashboard](screenshot.jpg)


## Usage ##

You can use the Dockerimage: `aoepeople/vistecture-dashboard:2.1.2`

### Example Project

```
docker run --rm -ti -p 8080:8080 aoepeople/vistecture-dashboard:2.1.2
```

### Custom Project
Just copy your vistecture definitions into /vistecture/project.yml

The following Dockerfile could be used to build an image running the dashboard for your defined architecture:

```dockerfile
FROM aoepeople/vistecture-dashboard:2.1.2

COPY definition /definition
CMD ["-config", "/definition/project.yml"]
```

### Vistecture Properties that are used:
The following "Properties" are used to control dashboard behaviour
(See example folder for an example)
- `deployment`: Has to be set to `kubernetes` (otherwise app is not checked)
- `healthCheckPath`: Healthcheck endpoint (relative path) (Optional - if not set just the base url is called) - If a healthCheckPath is configured it need to match the defined format (see below)
- `apiDocPath`: Optional the relative path to an API spec (just used to show a link)
- `k8sDeploymentName`: Override the name of the deployment in kubernetes that is checked(default = appname)
- `k8sHealthCheckServiceName`: Override service name that is used to check health (default = appname)
- `k8sHealthCheckThroughIngress`: If the app should be checked from public (ingress is required for the service)
- `k8sType`: set to "job" if the application is not represented by an deployment in kubernetes, but it is just a job

### Healtcheck Format:

If a Healthcheck path is configured for the application the following format is evaluated:

```json
{
"services": [
    {
        "name": "session",
        "alive": true,
        "details": "success"
    },
    {
        "name": "magento",
        "alive": true,
        "details": "magento is alive"
    },
    {
        "name": "om3oms-rabbitMQ-publisher",
        "alive": false,
        "details": "dial tcp [::1]:5672: connect: connection refused"
    }
]
}
```

## Development: ##

```
# run
go run vistecture-dashboard.go
```

For a demo display please use:
```
go run vistecture-dashboard.go -config=example/project.yml -Demo
```

And access it via http://localhost:8080
