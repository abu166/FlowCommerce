FROM golang:1.23

WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

COPY . . 

RUN CGO_ENABLED=0 GOOS=linux go build -o /cmd/marketflow

EXPOSE 8080

CMD ["/cmd/marketflow"]