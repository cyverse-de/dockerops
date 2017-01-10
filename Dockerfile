FROM golang:1.7-alpine

RUN apk update && apk add git

RUN go get github.com/jstemmer/go-junit-report

RUN go get github.com/docker/docker/client
RUN rm -r /go/src/github.com/docker/docker/vendor
RUN go get github.com/pkg/errors
RUN go get github.com/olebedev/config
RUN go get github.com/cyverse-de/logcabin
RUN go get github.com/cyverse-de/model
RUN go get github.com/cyverse-de/configurate
RUN go get github.com/docker/distribution/reference
RUN go get github.com/docker/docker/pkg/stdcopy
RUN go get github.com/docker/go-connections/nat
RUN go get github.com/docker/go-connections/sockets
RUN go get github.com/docker/go-connections/tlsconfig
RUN go get github.com/docker/go-units

COPY . /go/src/github.com/cyverse-de/dockerops

CMD go test -v github.com/cyverse-de/dockerops | tee /dev/stderr | go-junit-report
