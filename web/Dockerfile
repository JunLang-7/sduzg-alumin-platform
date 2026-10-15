FROM node:22-alpine AS builder

# 依赖源可用构建参数覆盖，便于国内服务器走镜像站（见 docker-compose.yml 与 .env.example）。
ARG NPM_REGISTRY=https://registry.npmjs.org

WORKDIR /app

COPY package*.json ./
RUN npm ci --registry="$NPM_REGISTRY"

COPY . .
RUN npm run build

FROM nginx:1.27-alpine

RUN rm -f /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
