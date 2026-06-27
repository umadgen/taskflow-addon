FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -o foyer-go .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/foyer-go .
ENV FOYER_DB=/data/foyer.sqlite \
    FOYER_STATIC=/app/public \
    PORT=8787
EXPOSE 8787
CMD ["./foyer-go"]
