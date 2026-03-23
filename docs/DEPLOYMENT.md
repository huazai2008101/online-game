# Game Platform Deployment Guide

## Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- 8GB RAM minimum (16GB recommended)
- 50GB disk space

## Quick Start

### 1. Start Infrastructure

```bash
./deploy/start.sh
```

This will start:
- 5 PostgreSQL databases
- Redis cache
- Kafka message queue
- NATS messaging
- Prometheus monitoring
- Grafana dashboards
- Jaeger tracing
- All 12 microservices
- API Gateway

### 2. Access Services

| Service | URL | Credentials |
|---------|-----|-------------|
| API Gateway | http://localhost:8080 | - |
| Grafana | http://localhost:3000 | admin/admin |
| Prometheus | http://localhost:9090 | - |
| Jaeger UI | http://localhost:16686 | - |

## Service Ports

| Service | Internal Port | External Port |
|---------|---------------|---------------|
| User Service | 8001 | 8001 |
| Game Service | 8002 | 8002 |
| Payment Service | 8003 | 8003 |
| Player Service | 8004 | 8004 |
| Activity Service | 8005 | 8005 |
| Guild Service | 8006 | 8006 |
| Item Service | 8007 | 8007 |
| Notification Service | 8008 | 8008 |
| Organization Service | 8009 | 8009 |
| Permission Service | 8010 | 8010 |
| ID Service | 8011 | 8011 |
| File Service | 8012 | 8012 |
| API Gateway | 8080 | 8080 |

## Management Commands

### View Logs

```bash
# View all logs
./deploy/logs.sh

# View specific service logs
./deploy/logs.sh game-service

# View last 500 lines
./deploy/logs.sh all -n 500
```

### Stop Services

```bash
# Stop all services
./deploy/stop.sh

# Stop and remove volumes
./deploy/stop.sh --volumes
```

### Restart Services

```bash
./deploy/restart.sh
```

## Database Access

### PostgreSQL

Connect to any database:

```bash
# Platform database
docker exec -it game-platform-db psql -U gameuser -d game_platform_db

# Core database
docker exec -it game-core-db psql -U gameuser -d game_core_db
```

### Redis

```bash
docker exec -it game-redis redis-cli
```

### Kafka

```bash
docker exec -it game-kafka kafka-topics --bootstrap-server localhost:9092 --list
```

## Health Checks

All services expose health endpoints:

```bash
# Service health
curl http://localhost:8001/health

# Liveness probe
curl http://localhost:8001/live

# Readiness probe
curl http://localhost:8001/ready
```

## Monitoring

### Prometheus Metrics

All services expose metrics at `/metrics`:

```bash
curl http://localhost:8001/metrics
```

### Grafana Dashboards

1. Access Grafana at http://localhost:3000
2. Login with admin/admin
3. Import dashboards from `deploy/grafana-dashboards/`

### Jaeger Tracing

Access Jaeger UI at http://localhost:16686 to view distributed traces.

## Configuration

### Environment Variables

Services can be configured via environment variables. See `docker-compose.yml` for examples.

### Custom Configuration

1. Copy `docker-compose.yml` to `docker-compose.override.yml`
2. Make your changes
3. Restart with `docker-compose -f docker-compose.yml -f docker-compose.override.yml up -d`

## Scaling

### Horizontal Scaling

```bash
# Scale game service to 3 instances
docker-compose -f deploy/docker-compose.yml -p game-platform up -d --scale game-service=3
```

### Load Balancing

For production deployment, use a proper load balancer (nginx, HAProxy, or cloud LB).

## Troubleshooting

### Service Won't Start

1. Check logs: `./deploy/logs.sh <service-name>`
2. Verify dependencies are running: `docker-compose ps`
3. Check resource usage: `docker stats`

### Database Connection Issues

1. Verify database is healthy: `docker exec game-platform-db pg_isready`
2. Check connection settings in environment variables
3. Review database logs

### High Memory Usage

1. Check container stats: `docker stats`
2. Adjust resource limits in `docker-compose.yml`
3. Scale down services if needed

## Production Considerations

### Security

1. Change default passwords
2. Use secrets management (Vault, AWS Secrets Manager)
3. Enable TLS/SSL
4. Configure proper CORS policies
5. Implement rate limiting

### Performance

1. Enable connection pooling
2. Configure Redis caching
3. Use CDN for static assets
4. Enable GZIP compression
5. Monitor and optimize slow queries

### High Availability

1. Use managed database services (RDS, Cloud SQL)
2. Enable Redis cluster mode
3. Configure Kafka replication
4. Implement health checks and auto-restart
5. Use multiple availability zones

### Backup

1. Regular database backups
2. Backup Redis persistence files
3. Document configuration changes
4. Test disaster recovery procedures

## Upgrading

### Rolling Update

```bash
# Pull new images
docker-compose -f deploy/docker-compose.yml -p game-platform pull

# Restart services one by one
docker-compose -f deploy/docker-compose.yml -p game-platform up -d --no-deps user-service
```

### Database Migration

```bash
# Run migrations
docker-compose -f deploy/docker-compose.yml -p game-platform run --rm user-service ./migrate up
```
