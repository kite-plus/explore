# The Explore frontend: Astro rendered on the server by Node.

# Must match .nvmrc.
ARG NODE_VERSION=22.19.0

FROM node:${NODE_VERSION}-alpine AS build
WORKDIR /src

# The manifest and lockfile first, so dependencies are cached across builds.
COPY package.json pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile

COPY . .
RUN pnpm build && pnpm prune --prod

FROM node:${NODE_VERSION}-alpine

WORKDIR /app
ENV NODE_ENV=production \
    HOST=0.0.0.0 \
    PORT=4321

COPY --from=build /src/package.json ./
COPY --from=build /src/node_modules ./node_modules
COPY --from=build /src/dist ./dist

USER node
EXPOSE 4321
CMD ["node", "dist/server/entry.mjs"]
