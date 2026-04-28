# Shared Next.js Dockerfile.
#
# Build: docker build -f app/web/Dockerfile --build-arg APP=www -t llmhub-www app/web
# APP must be one of: www / console / admin.

ARG NODE_VERSION=20
ARG APP

FROM node:${NODE_VERSION}-alpine AS deps
ARG APP
WORKDIR /work
RUN corepack enable && corepack prepare pnpm@9 --activate
COPY pnpm-workspace.yaml ./
COPY ${APP}/package.json ${APP}/package.json
RUN cd ${APP} && pnpm install --no-frozen-lockfile

FROM node:${NODE_VERSION}-alpine AS build
ARG APP
WORKDIR /work
RUN corepack enable && corepack prepare pnpm@9 --activate
COPY --from=deps /work/${APP}/node_modules ${APP}/node_modules
COPY pnpm-workspace.yaml ./
COPY ${APP} ${APP}
RUN cd ${APP} && pnpm build

FROM node:${NODE_VERSION}-alpine AS runtime
ARG APP
ENV NODE_ENV=production
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9 --activate
COPY --from=build /work/${APP}/.next ./.next
COPY --from=build /work/${APP}/package.json ./package.json
COPY --from=build /work/${APP}/node_modules ./node_modules
COPY --from=build /work/${APP}/next.config.ts ./next.config.ts
USER node
EXPOSE 3000
CMD ["pnpm", "start"]
