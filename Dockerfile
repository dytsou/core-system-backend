FROM golang:1.24

LABEL log.enable="true"

WORKDIR /app

COPY --from=builder /app/bin/backend /app/backend
COPY internal/database/migrations /app/migrations


EXPOSE 8080

CMD ["/app/backend"]