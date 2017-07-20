FROM golang:1.8-alpine

ADD . /go/src/github.com/AOEpeople/vistecture-dashboard

RUN apk update && apk add git
RUN go get -u github.com/golang/dep/cmd/dep
RUN cd /go/src/github.com/AOEpeople/vistecture-dashboard \
    && dep ensure -v \
    && go install .
