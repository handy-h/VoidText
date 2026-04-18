FROM golang:1.20-alpine as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o txtclean ./cmd/txtclean

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/txtclean .
COPY configs/ ./configs/
COPY models/ ./models/

RUN mkdir -p data

EXPOSE 8080

CMD ["./txtclean"]