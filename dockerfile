FROM golang:1.20-alpine
RUN mkdir /app
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o file_server .
RUN mkdir -p /storage
VOLUME /storage
EXPOSE 8080
CMD ["./file_server"]