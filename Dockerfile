FROM golang:alpine AS builder
RUN apk update && apk add --no-cache ca-certificates git && update-ca-certificates
COPY . /app
RUN cd /app && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/app /app
COPY templates /templates
COPY example /example
EXPOSE 8080

ENTRYPOINT ["/app"]
CMD ["-Demo"]
