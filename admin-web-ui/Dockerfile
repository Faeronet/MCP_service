# Use ghcr.io if Docker Hub is unreachable: --build-arg REGISTRY=ghcr.io/docker-library/
ARG REGISTRY=
FROM ${REGISTRY}node:20-alpine AS builder
WORKDIR /app
COPY admin-web-ui/package.json admin-web-ui/package-lock.json* admin-web-ui/ ./
RUN npm install
COPY admin-web-ui/ ./
ARG VITE_API_URL=http://localhost:8080
ENV VITE_API_URL=$VITE_API_URL
RUN npm run build

FROM ${REGISTRY}nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY admin-web-ui/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
