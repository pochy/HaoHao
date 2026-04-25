FROM node:24 AS frontend-builder
WORKDIR /app
COPY frontend/package*.json ./frontend/
RUN npm --prefix frontend ci
COPY frontend ./frontend
RUN npm --prefix frontend run build

FROM golang:1.26 AS backend-builder
WORKDIR /app
COPY go.work ./
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend ./backend
COPY --from=frontend-builder /app/backend/web/dist ./backend/web/dist
RUN GOMEMLIMIT=512MiB GOGC=25 CGO_ENABLED=0 go build -p 1 -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o /tmp/haohao ./backend/cmd/main

FROM scratch
COPY --from=backend-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend-builder /tmp/haohao /haohao
USER 65532:65532
EXPOSE 8080
CMD ["/haohao"]
