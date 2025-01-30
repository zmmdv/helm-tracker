FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

WORKDIR /app

# Install git and dependencies
RUN apk add --no-cache git

# Copy go mod files first
COPY go.mod ./
RUN go mod download
RUN go mod verify

# Copy source code
COPY . .

# Initialize and verify modules
RUN go mod tidy

# Build the application
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o helm-monitor ./cmd/helm-monitor

FROM --platform=$TARGETPLATFORM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/helm-monitor /helm-monitor

ENTRYPOINT ["/helm-monitor"]