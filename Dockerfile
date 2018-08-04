ARG repo_arch
FROM ${repo_arch}golang:1.10 AS builder

RUN go get -u github.com/golang/dep/cmd/dep

WORKDIR /go/src/app/
COPY ./Gopkg.* /go/src/app/
RUN dep ensure --vendor-only

COPY ./main.go /go/src/app/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o dsched .

FROM ${repo_arch}busybox:latest
WORKDIR /root/
COPY --from=builder /go/src/app/dsched .

CMD [ "./dsched" ]
