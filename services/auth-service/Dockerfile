FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json tsconfig.json ./
RUN npm install
COPY src ./src
RUN npm run build
# Ensure proto file is in dist for runtime gRPC loading
RUN mkdir -p dist/protos && cp src/protos/auth.proto dist/protos/

FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install --omit=dev
COPY --from=builder /app/dist ./dist
EXPOSE 8004
CMD ["node", "dist/index.js"]
