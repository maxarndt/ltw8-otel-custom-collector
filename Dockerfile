# Stage 1: Build the custom collector binary using ocb
FROM golang:1.25 AS builder

WORKDIR /build

# Install ocb (OpenTelemetry Collector Builder)
RUN GOBIN=/usr/local/bin go install go.opentelemetry.io/collector/cmd/builder@v0.149.0

# Copy the module source
COPY receiver/ receiver/
COPY collector/builder-config.yaml collector/builder-config.yaml

# Build the collector
RUN cd collector && builder --config builder-config.yaml --skip-compilation=false

# Stage 2: Minimal runtime image
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /build/collector/build/knx-collector /knx-collector

EXPOSE 13133

ENTRYPOINT ["/knx-collector"]
CMD ["--config", "/etc/otelcol/collector-config.yaml"]
