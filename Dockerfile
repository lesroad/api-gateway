FROM registry.cn-shanghai.aliyuncs.com/lesroad/infrastructure:golang_1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api-gateway .

FROM registry.cn-shanghai.aliyuncs.com/lesroad/infrastructure:alpine_3.18

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/api-gateway .

EXPOSE 8070

CMD ["./api-gateway"]