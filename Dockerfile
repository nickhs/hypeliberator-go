FROM golang:1-alpine3.24 AS build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 go build -o hypeliberator-go

FROM alpine:3.24

RUN apk add --no-cache ca-certificates

COPY ./static/ /srv/static
COPY ./index.html /srv/index.html
COPY --from=build /build/hypeliberator-go /srv/hypeliberator-go

WORKDIR /srv
EXPOSE 4567
CMD ["./hypeliberator-go"]
