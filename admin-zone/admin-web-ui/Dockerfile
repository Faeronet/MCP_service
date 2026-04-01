FROM node:20-slim AS builder
WORKDIR /app
COPY admin-web-ui/package.json admin-web-ui/package-lock.json* ./
RUN npm install
COPY admin-web-ui/ ./
ARG VITE_API_URL=
ENV VITE_API_URL=$VITE_API_URL
ARG VITE_MONITOR_POLL_MS=30000
ENV VITE_MONITOR_POLL_MS=$VITE_MONITOR_POLL_MS
ENV NODE_OPTIONS="--max-old-space-size=4096"
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY admin-web-ui/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
