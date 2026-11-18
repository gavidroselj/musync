ARG GO_VERSION=1.27.4

# Build app

FROM golang:${GO_VERSION} AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o musync ./cmd/musync

# Download dependencies

FROM golang:${GO_VERSION} AS dependencies
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go run ./cmd/download-dependencies

# Assemble final image

FROM denoland/deno
WORKDIR /app/

# Copy cached dependencies
COPY --from=dependencies /root/.cache /root/.cache

COPY --from=build /app/musync .

CMD [ "/app/musync" ]