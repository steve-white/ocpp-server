FROM golang:1.25-alpine AS builder
# Install build dependencies for CGO (required for SQLite)
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app
COPY . .
# Enable CGO and build binaries inside the Linux container
ENV CGO_ENABLED=1

# Set build arguments for version information
ARG BUILD_NUMBER="docker-build"
ARG BUILD_SOURCE_VERSION="unknown"
ARG BUILD_TIMESTAMP=""

# Build with ldflags for version information
RUN cd src && go build --ldflags="-X sw/ocpp/csms/internal/service.Version=${BUILD_NUMBER} -X sw/ocpp/csms/internal/service.CommitHash=${BUILD_SOURCE_VERSION} -X sw/ocpp/csms/internal/service.BuildTimestamp=${BUILD_TIMESTAMP}" -o csms-server ./csms-server/
RUN cd src && go build --ldflags="-X sw/ocpp/csms/internal/service.Version=${BUILD_NUMBER} -X sw/ocpp/csms/internal/service.CommitHash=${BUILD_SOURCE_VERSION} -X sw/ocpp/csms/internal/service.BuildTimestamp=${BUILD_TIMESTAMP}" -o device-manager ./device-manager/
RUN cd src && go build --ldflags="-X sw/ocpp/csms/internal/service.Version=${BUILD_NUMBER} -X sw/ocpp/csms/internal/service.CommitHash=${BUILD_SOURCE_VERSION} -X sw/ocpp/csms/internal/service.BuildTimestamp=${BUILD_TIMESTAMP}" -o message-manager ./message-manager/
RUN cd src && go build --ldflags="-X sw/ocpp/csms/internal/service.Version=${BUILD_NUMBER} -X sw/ocpp/csms/internal/service.CommitHash=${BUILD_SOURCE_VERSION} -X sw/ocpp/csms/internal/service.BuildTimestamp=${BUILD_TIMESTAMP}" -o session ./session/

FROM nginx:alpine
RUN apk add --no-cache supervisor

# Create required directories first
RUN mkdir -p /usr/local/csms-server/bin /usr/local/csms-server/cfg /usr/local/csms-server/db

# Copy built binaries from builder stage
COPY --from=builder /app/src/csms-server/csms-server /usr/local/csms-server/bin
COPY --from=builder /app/src/device-manager/device-manager /usr/local/csms-server/bin
COPY --from=builder /app/src/message-manager/message-manager /usr/local/csms-server/bin
COPY --from=builder /app/src/session/session /usr/local/csms-server/bin

# Copy configuration files
COPY src/cfg/conf.example.yaml /usr/local/csms-server/cfg/conf.yaml
COPY deploy/supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY deploy/nginx.conf /etc/nginx/nginx.conf

EXPOSE 5580
EXPOSE 3007
CMD ["supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]