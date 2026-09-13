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

FROM scratch AS ingestion
COPY --from=ingestion-build /ingest /ingest
USER 65532:65532
ENTRYPOINT ["/ingest"]

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
