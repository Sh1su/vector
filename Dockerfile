# Vectra – ein Image mit Backend und Web-App (ADR-030). Laufzeit: FROM scratch, ohne Root.
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
COPY api/ /src/api/
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src/backend
ENV CGO_ENABLED=0 GOFLAGS=-mod=mod
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN go build -trimpath -ldflags "-s -w" -o /out/vectra ./cmd/vectra

FROM scratch
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend /out/vectra /vectra
COPY --from=web /src/web/dist /web
ENV VECTRA_WEB_DIR=/web VECTRA_LISTEN=:8080
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/vectra"]
