# syntax=docker/dockerfile:1

# Vite bakes the VITE_* variables into the bundle at build time, so every one is a build arg and one image serves one
# configuration. An empty arg counts as unset: the redirect URIs then default to <origin>/callback and <origin>/.
FROM node:22-alpine AS build
WORKDIR /web
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG VITE_NICE_ISSUER
ARG VITE_NICE_CLIENT_ID
ARG VITE_REDIRECT_URI
ARG VITE_POST_LOGOUT_REDIRECT_URI
ARG VITE_SERVICES
ENV VITE_NICE_ISSUER=$VITE_NICE_ISSUER \
    VITE_NICE_CLIENT_ID=$VITE_NICE_CLIENT_ID \
    VITE_REDIRECT_URI=$VITE_REDIRECT_URI \
    VITE_POST_LOGOUT_REDIRECT_URI=$VITE_POST_LOGOUT_REDIRECT_URI \
    VITE_SERVICES=$VITE_SERVICES
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.30-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /web/dist /usr/share/nginx/html
EXPOSE 8080
