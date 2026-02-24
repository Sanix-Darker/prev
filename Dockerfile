ARG GO_VERSION=1.24

# Build stage
FROM golang:${GO_VERSION} AS builder

ARG GIT_COMMIT
ARG VERSION

ENV GO111MODULE=on
ENV CGO_ENABLED=0

WORKDIR $GOPATH/src/github.com/sanix-darker/prev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go generate ./... && go build -o /go/bin/prev
RUN echo "nonroot:x:65534:65534:Non root:/:" > /etc_passwd


# Final stage
FROM scratch

LABEL maintainer="sanix-darker <s4nixd@gmail.com>"

COPY --from=builder /go/bin/prev /bin/prev
COPY --from=builder /etc_passwd /etc/passwd

USER nonroot

ENTRYPOINT [ "prev" ]
CMD [ "version" ]
