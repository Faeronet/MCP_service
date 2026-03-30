# Сборка из корня репозитория: docker build -f angels-web/Dockerfile .
FROM node:20-alpine
WORKDIR /app
COPY angels-web/package.json ./
RUN npm install
COPY angels-web/ ./
RUN npm run build
EXPOSE 3000
ENV NODE_ENV=production
CMD ["npm", "run", "start"]
