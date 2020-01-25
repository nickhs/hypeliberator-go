FROM golang:1.12-alpine3.9 AS build

RUN apk add --no-cache git

RUN go get github.com/mitchellh/mapstructure 

RUN mkdir /build
WORKDIR /build
COPY main.go .
RUN go build -o hypeliberator-go

FROM alpine:3.9

RUN apk add --no-cache ca-certificates

COPY ./static/ /srv/static
COPY ./index.html /srv/index.html
COPY --from=build /build/hypeliberator-go /srv/hypeliberator-go

WORKDIR /srv
EXPOSE 4567
CMD ./hypeliberator-go
