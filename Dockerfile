FROM golang:1.15.8-alpine as builder

RUN echo http://mirrors.aliyun.com/alpine/latest-stable/main/ > /etc/apk/repositories \
 && echo http://mirrors.aliyun.com/alpine/latest-stable/community/ >> /etc/apk/repositories \
 && apk add git \
 && go get -v github.com/docker/docker/client

COPY ./main.go /app/main.go
RUN cd /app \
 && go build -o ditree

FROM alpine

COPY --from=builder /app/ditree /usr/local/bin/

CMD ["ditree"]
