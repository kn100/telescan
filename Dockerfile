FROM golang AS build
WORKDIR /app
COPY go.* /app/
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o telescan 

FROM debian:bookworm-slim AS run
RUN apt-get update && apt-get install -y ca-certificates
RUN rm -rf /var/lib/apt/lists/*
COPY --from=build /app/telescan /telescan
ENTRYPOINT ["/telescan"]