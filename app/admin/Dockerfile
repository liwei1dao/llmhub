# Single Next.js app: marketing + console + admin under one server.
ARG NODE_VERSION=22

FROM node:${NODE_VERSION}-alpine AS deps
ENV COREPACK_NPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9 --activate
RUN pnpm config set registry https://registry.npmmirror.com
COPY package.json ./
RUN pnpm install --no-frozen-lockfile

FROM node:${NODE_VERSION}-alpine AS build
ENV COREPACK_NPM_REGISTRY=https://registry.npmmirror.com
ARG NEXT_PUBLIC_API_BASE=http://localhost:8080
ENV NEXT_PUBLIC_API_BASE=${NEXT_PUBLIC_API_BASE}
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9 --activate
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN pnpm build

FROM node:${NODE_VERSION}-alpine AS runtime
ENV NODE_ENV=production
ENV COREPACK_NPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9 --activate
COPY --from=build /app/.next ./.next
COPY --from=build /app/package.json ./package.json
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/next.config.ts ./next.config.ts
USER node
EXPOSE 3000
CMD ["pnpm", "start"]
