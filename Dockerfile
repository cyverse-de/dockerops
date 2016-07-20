FROM golang:1.6-alpine

RUN apk update && apk add git

RUN go get github.com/docker/engine-api
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
RUN go get golang.org/x/net/context

COPY . /go/src/github.com/cyverse-de/dockerops

CMD ["go", "test", "github.com/cyverse-de/dockerops"]
