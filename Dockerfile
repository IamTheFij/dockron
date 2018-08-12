FROM golang:1.10 AS builder

RUN go get -u github.com/golang/dep/cmd/dep

WORKDIR /go/src/app/
COPY ./Gopkg.* /go/src/app/
RUN dep ensure --vendor-only

COPY ./main.go /go/src/app/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o dockron .

FROM busybox:latest
WORKDIR /root/
COPY --from=builder /go/src/app/dockron .

CMD [ "./dockron" ]
