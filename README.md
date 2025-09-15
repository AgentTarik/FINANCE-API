# Finance API

RESTful API in Go that simulates a financial transactions system with:

- **PostgreSQL** for persistence
- **Asynchronous Processing** (internal worker)  
- **Kafka Events** (`transactions.created`)  
- **JWT authentication** (Bearer)  
- **Observability**: Prometheus (+ metrics at `/metrics`) and Grafana  
- **tools**: Adminer (DB) and Kafka UI (topics/messages)  
- **Simple Consumer** (separate service) that reads from Kafka

> This README explains how to run and test the project with Docker.

---

## 🔧 Prerequisites

- Docker Desktop (or Docker Engine + Docker Compose Plugin)
- (Optional, to rebuild docs) Go 1.22+ and swag CLI

---

## 🚀 Spin everything up with Docker

From Root:

```
docker compose up -d --build postgres kafka kafka-ui adminer api consumer prometheus grafana
docker compose ps
```
