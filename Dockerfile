# Vectra – ein Image mit Backend und Web-App (ADR-030). Laufzeit: FROM scratch, ohne Root.
# Web und Go laufen auf der Build-Plattform; Go übersetzt direkt für die Zielarchitektur (amd64, arm64).
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
COPY api/ /src/api/
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend
ARG TARGETOS=linux TARGETARCH=amd64
WORKDIR /src/backend
ENV CGO_ENABLED=0 GOFLAGS=-mod=mod
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w" -o /out/vectra ./cmd/vectra && mkdir -p /out/data/files

FROM scratch
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend /out/vectra /vectra
COPY --from=web /src/web/dist /web
# Dateiablage (ADR-017): eigenes Volume, gehört dem Laufzeitnutzer
COPY --from=backend --chown=65532:65532 /out/data /data
ENV VECTRA_WEB_DIR=/web VECTRA_LISTEN=:8080 VECTRA_STORAGE_DIR=/data/files GOMEMLIMIT=100MiB
VOLUME /data
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/vectra"]
