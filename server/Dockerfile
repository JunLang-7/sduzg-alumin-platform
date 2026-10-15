FROM golang:1.26.9-alpine AS builder

# 依赖源可用构建参数覆盖，便于国内服务器走镜像站（见 docker-compose.yml 与 .env.example）。
# APK_MIRROR 留空表示使用 Alpine 官方源；非空时把 dl-cdn.alpinelinux.org 换成该主机名。
ARG APK_MIRROR=
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY

RUN if [ -n "$APK_MIRROR" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi

WORKDIR /src

RUN apk add --no-cache ca-certificates git tzdata

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/api

FROM alpine:3.20

ARG APK_MIRROR=
RUN if [ -n "$APK_MIRROR" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S app -G app

ENV GIN_MODE=release
WORKDIR /app

COPY --from=builder /out/server /app/server

EXPOSE 8080
USER app

ENTRYPOINT ["/app/server"]
