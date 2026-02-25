# Используем Debian-slim: в Alpine при tsc/vite build иногда возникает segfault (exit 139)
FROM node:20-slim AS builder
WORKDIR /app
COPY admin-web-ui/package.json admin-web-ui/package-lock.json* admin-web-ui/ ./
RUN npm install
COPY admin-web-ui/ ./
# Пустой URL = относительные запросы /api/..., nginx проксирует на admin-backend
ARG VITE_API_URL=
ENV VITE_API_URL=$VITE_API_URL
# Увеличиваем heap Node (segfault при сборке часто из-за нехватки памяти в контейнере)
ENV NODE_OPTIONS="--max-old-space-size=4096"
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY admin-web-ui/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
