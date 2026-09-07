# callhook — single-binary deploy.
# Builds the web app, embeds it, and produces the server binary.
# Used by Render (and any Docker host).

FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web ./
RUN npm run build
# Fail loudly here if the build produced nothing (catches config drift).
RUN test -d dist && test -f dist/index.html

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY backend/go.mod ./
COPY backend/ ./
# Embed the freshly built frontend into the binary (make full equivalent).
# COPY --from=web is what actually triggers the web stage to build — a plain
# `cp /web/dist/*` would leave the stage pruned and the copy empty.
COPY --from=web /web/dist ./internal/api/webdist
RUN touch internal/api/webdist/.gitkeep
RUN CGO_ENABLED=0 go build -o /out/callhook ./cmd/callhook \
    && CGO_ENABLED=0 go build -o /out/callhookctl ./cmd/callhookctl

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 callhook
WORKDIR /app
COPY --from=build /out/callhook /app/callhook
COPY --from=build /out/callhookctl /app/callhookctl
# data/ is the journal location (mount a disk or it resets on redeploy).
RUN mkdir -p /app/data && chown -R callhook:callhook /app
USER callhook
ENV CALLHOOK_JOURNAL=/app/data/sessions.jsonl \
    CALLHOOK_CAMPAIGN_JOURNAL=/app/data/campaigns.jsonl
EXPOSE 8080
ENTRYPOINT ["/app/callhook"]
