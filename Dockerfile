FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /cookiekill .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 10001 --create-home cookie && mkdir /data && chown cookie:cookie /data
RUN mkdir /app
COPY --from=build /cookiekill /app/cookiekill
USER cookie
WORKDIR /data
EXPOSE 80 443
# Mount the JSON configuration at /app/config.json, with dataDir=/data and
# logFile=/data/logs/cookiekill.log so the unprivileged user can write both.
ENTRYPOINT ["/app/cookiekill"]
