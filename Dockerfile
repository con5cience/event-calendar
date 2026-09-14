FROM node:24-bookworm-slim AS frontend
WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json vite.config.ts ./
COPY web ./web
COPY internal/artifact/schema ./internal/artifact/schema
COPY tests/contracts ./tests/contracts
RUN npm run build

FROM golang:1.26-bookworm AS go-base
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

FROM go-base AS go-test
COPY tests/contracts ./tests/contracts
RUN test -z "$(gofmt -l cmd internal)" && go vet ./... && go test -race ./...

FROM go-base AS backend
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /calendar ./cmd/calendar

FROM go-base AS ingestion-build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /ingest ./cmd/ingest
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /package-snapshot ./cmd/package-snapshot

FROM scratch AS ingestion
COPY --from=ingestion-build /ingest /ingest
USER 65532:65532
ENTRYPOINT ["/ingest"]

FROM node:24-bookworm-slim AS refresh
WORKDIR /tools
COPY package.json package-lock.json ./
RUN npm ci && npx playwright install --with-deps chromium
COPY scripts ./scripts
COPY tests ./tests
COPY --from=ingestion-build /ingest /usr/local/bin/ingest
COPY --from=ingestion-build /package-snapshot /usr/local/bin/package-snapshot
ENV CALENDAR_REPO=/repo
ENTRYPOINT ["node", "/tools/scripts/refresh-locale.mjs"]

FROM refresh AS refresh-test
COPY internal /fixtures/internal
COPY locales/denver/site.json /fixtures/locales/denver/site.json
COPY locales/denver/assets /fixtures/locales/denver/assets
COPY --from=backend /calendar /calendar
COPY --from=frontend /build/dist /app/dist
ENTRYPOINT ["node", "--test", "/tools/tests/refresh-integration.test.mjs"]

FROM node:24-bookworm-slim AS proxy-diagnostics
WORKDIR /tools
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates
COPY package.json package-lock.json ./
RUN npm ci
COPY scripts ./scripts
COPY tests ./tests
COPY .github/workflows/proxy-diagnostics.yml ./.github/workflows/proxy-diagnostics.yml
COPY locales/denver/site.json locales/denver/capture.json /site/
ENV SITE_DIR=/site
USER node
ENTRYPOINT ["node", "/tools/scripts/proxy-diagnostics.mjs"]

FROM scratch AS runtime
ARG LOCALE
COPY --from=backend /calendar /calendar
COPY --from=frontend /build/dist /app/dist
COPY locales/${LOCALE}/site.json /app/site/site.json
COPY locales/${LOCALE}/assets/ /app/site/assets/
USER 65532:65532
ENV PORT=8080 ASSETS_DIR=/app/dist DATA_DIR=/data SITE_DIR=/app/site
EXPOSE 8080
ENTRYPOINT ["/calendar"]
