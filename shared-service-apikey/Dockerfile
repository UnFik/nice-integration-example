# syntax=docker/dockerfile:1

# Node 22.18+ runs the .ts sources directly through type stripping, so there is no build stage and the dev
# dependencies (typescript, vitest) stay out of the image.
FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev && npm cache clean --force
COPY src ./src
# The key file is written atomically (temporary file + rename in the same directory), so /data must be writable by
# the user of the process.
RUN mkdir /data && chown node:node /data
USER node
# Loopback inside a container is unreachable from NICE, so the image listens on every interface of the container.
# Keep the port itself private: publish it only to the network NICE is on.
ENV NODE_ENV=production LISTEN_ADDR=0.0.0.0:8082 KEYS_FILE=/data/keys.json
VOLUME ["/data"]
EXPOSE 8082
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8082/healthz || exit 1
# node, not npm start: signals reach the process, and no .env is read inside the container.
CMD ["node", "src/index.ts"]
