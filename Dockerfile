FROM golang:1.25-alpine AS build

ARG TARGETARCH

ARG RISOR_VERSION="dev"
ARG GIT_REVISION="-"
ARG BUILD_DATE="-"

WORKDIR /app

COPY . .
RUN cd cmd/risor && go mod download
RUN CGO_ENABLED=0 GOOS=linux \
    go build \
    -ldflags "-X 'main.version=${RISOR_VERSION}' -X 'main.commit=${GIT_REVISION}' -X 'main.date=${BUILD_DATE}'" \
    -o risor \
    ./cmd/risor

FROM alpine:3.23

WORKDIR /app

COPY --from=build /app/risor /usr/local/bin/risor
RUN apk --no-cache add ca-certificates tzdata

ENTRYPOINT ["risor"]
