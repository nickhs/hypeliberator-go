FROM alpine

RUN apk add --no-cache ca-certificates

COPY ./static/ /srv/static
COPY ./index.html /srv/index.html
COPY ./build/hypeliberator-go-linux-amd64 /srv/hypeliberator-go

WORKDIR /srv
EXPOSE 4567
CMD ./hypeliberator-go
