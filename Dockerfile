ARG GO_VERSION=1.27.4

ARG UID=1000
ARG GID=1000

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


# Run as non-root user
ARG UID
ARG GID

RUN groupadd -g $UID musync && \
    useradd -m -u $GID -g musync musync
USER ${UID}:${GID}

# Copy cached dependencies
COPY --chown=${UID}:${GID} --from=dependencies /root/.cache /home/musync/.cache

COPY --chown=${UID}:${GID} --from=build /app/musync .

CMD [ "/app/musync" ]