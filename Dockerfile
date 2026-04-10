# AgentWiki Dockerfile
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git make gcc g++ musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o agentwiki ./cmd/agentwiki
RUN CGO_ENABLED=0 go build -o awctl ./cmd/awctl

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/agentwiki /usr/local/bin/
COPY --from=builder /app/awctl /usr/local/bin/
COPY web/ /app/web/

EXPOSE 8080 9000

CMD ["agentwiki", "serve"]
