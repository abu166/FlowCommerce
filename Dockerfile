FROM golang:1.23

WORKDIR /cmd/marketflow

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

COPY . . 

RUN go build -o app ./cmd/marketflow/main.go

EXPOSE 8080

CMD ["./app"]