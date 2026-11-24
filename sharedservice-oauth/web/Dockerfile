# syntax=docker/dockerfile:1

# The UI is static files. nginx serves them and passes /auth and /devices to the backend, as the Vite dev server
# does, so the browser sees one origin (nginx.conf.template).
FROM node:22-alpine AS build
WORKDIR /web
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.30-alpine
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /web/dist /usr/share/nginx/html
# BACKEND_ADDR is the host:port of the backend. NGINX_ENTRYPOINT_LOCAL_RESOLVERS gives the template the resolver of
# the container, so the backend name is looked up on each request.
ENV BACKEND_ADDR=nadi:8081 NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1
EXPOSE 8080
