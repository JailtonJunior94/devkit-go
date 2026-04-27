FROM golang:1.26.2-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /bin/lgtm-demo ./pkg/observability/examples/lgtm-demo

FROM alpine:3.22
RUN addgroup -S app && adduser -S app -G app
USER app

COPY --from=build /bin/lgtm-demo /usr/local/bin/lgtm-demo
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/lgtm-demo"]
