FROM golang:1.20-alpine

# Устанавливаем рабочую директорию
WORKDIR /go/src/file_server
COPY . .
RUN go mod init 
RUN go build .
# Создаем папку для хранения данных и назначаем volume
RUN mkdir -p /storage
VOLUME /storage

# Пробрасываем порт 8080
EXPOSE 8080

# Запускаем приложение
CMD ["./file_server"]