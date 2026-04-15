# Stage 1: Build the custom collector binary using ocb
FROM golang:1.25 AS builder

WORKDIR /build

# Install ocb (OpenTelemetry Collector Builder)
RUN GOBIN=/usr/local/bin go install go.opentelemetry.io/collector/cmd/builder@v0.149.0

# Copy the module source
COPY receiver/ receiver/
COPY collector/builder-config.yaml collector/builder-config.yaml

# Build the collector.
# GOWORK=off: das OCB-Build-Verzeichnis ist kein go.work-Modul — ohne dieses
# Flag schlägt 'go build' fehl wenn go.work im Parent-Verzeichnis vorhanden ist.
RUN cd collector && GOWORK=off builder --config builder-config.yaml --skip-compilation=false

# Stage 2: Minimal runtime image
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /build/collector/build/custom-collector /custom-collector

EXPOSE 13133

ENTRYPOINT ["/custom-collector"]
CMD ["--config", "/etc/otelcol/collector-config.yaml"]
