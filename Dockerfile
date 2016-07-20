FROM golang:1.6-alpine

RUN apk update && apk add git

RUN go get github.com/fsouza/go-dockerclient
RUN go get github.com/olebedev/config
RUN go get github.com/cyverse-de/logcabin
RUN go get github.com/cyverse-de/model
RUN go get github.com/cyverse-de/configurate

COPY . /go/src/github.com/cyverse-de/dockerops

CMD ["go", "test", "github.com/cyverse-de/dockerops"]
