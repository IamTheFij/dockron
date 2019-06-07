ARG REPO=library
# FROM golang:1.11-alpine AS builder
#
# RUN apk add --no-cache git
# RUN go get -u github.com/golang/dep/cmd/dep
#
# WORKDIR /go/src/app/
# COPY ./Gopkg.* /go/src/app/
# RUN dep ensure --vendor-only
#
# COPY ./main.go /go/src/app/
#
# RUN CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -a -installsuffix nocgo -o dockron .

FROM ${REPO}/busybox:latest
WORKDIR /root/
# COPY --from=builder /go/src/app/dockron .
ARG ARCH=amd64
COPY ./dockron-linux-${ARCH} .

CMD [ "./dockron" ]
