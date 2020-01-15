FROM alpine

COPY templates /templates
COPY vistecture-dashboard /

EXPOSE 8080

WORKDIR /
ENTRYPOINT ["/vistecture-dashboard"]

