FROM node:24-alpine
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY package.json pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile
EXPOSE 5005
CMD ["pnpm", "dev", "--host", "0.0.0.0", "--port", "5005"]
