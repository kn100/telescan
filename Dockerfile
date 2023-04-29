# syntax=docker/dockerfile:1
   
FROM golang:alpine
WORKDIR /app
COPY . .
RUN apk add --no-cache sane-dev gcc libc-dev imagemagick-dev
ENV CGO_ENABLED=1
RUN go build -o telescan .

FROM alpine
RUN apk add --no-cache sane imagemagick
COPY --from=0 /app/telescan /usr/local/bin/telescan
ENTRYPOINT ["/usr/local/bin/telescan"]
