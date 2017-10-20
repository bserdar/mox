FROM golang:1.8-jessie

ENV APP_DIR /go/src/github.com/bserdar/mox/

WORKDIR $APP_DIR
COPY . $APP_DIR
RUN go get github.com/golang/lint/golint
RUN make

EXPOSE 8000
EXPOSE 8001
CMD ["bin/mox"]
