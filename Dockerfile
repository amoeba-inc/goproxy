FROM golang:1.12.9-alpine3.10 AS build

WORKDIR /goproxy

COPY . .

RUN go build -o goproxy

FROM golang:1.12.9-alpine3.10

RUN echo "Asia/Shanghai" > /etc/timezone \
    && apk add --no-cache -U \
    git \
    mercurial \
    subversion \
    bzr \
    fossil

COPY --from=build /goproxy/goproxy /goproxy/

EXPOSE 8080

CMD ["/goproxy/goproxy","-listen=0.0.0.0:8080"]