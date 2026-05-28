FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/http .

FROM gcr.io/distroless/static
COPY --from=build /out/http /http
EXPOSE 9003
ENTRYPOINT ["/http"]
