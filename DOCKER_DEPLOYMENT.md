# Docker Deployment Guide for Anchor

This guide provides comprehensive instructions for deploying the Anchor application using Docker.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose 2.0+
- Git

## Quick Start with Docker Compose

1. **Clone the repository and navigate to the project root:**
   ```bash
   git clone <repository-url>
   cd anchor
   ```

2. **Create environment file:**
   ```bash
   cp .env.example .env
   ```

3. **Edit the `.env` file with required values:**
   ```bash
   # Required - Database credentials
   POSTGRES_USER=anchor
   POSTGRES_PASSWORD=your_secure_database_password
   POSTGRES_DB=anchor

   # Required - JWT secret for authentication
   ADMIN_JWT_SECRET=your_very_secure_jwt_secret_key_here

   # Optional - Application settings
   ENVIRONMENT=production
   SERVER_PORT=8080
   POSTGRES_SSLMODE=disable
   REFRESH_TOKEN_LIFETIME=84600
   ACCESS_TOKEN_LIFETIME=3600
   ```

4. **Start the application:**
   ```bash
   docker-compose up -d
   ```

5. **Check application status:**
   ```bash
   docker-compose ps
   docker-compose logs anchor
   ```

6. **Access the application:**
   - API: http://localhost:8080
   - Health check: http://localhost:8080/health

## Manual Docker Run Example

If you prefer to run containers manually instead of using Docker Compose:

### 1. Create a Network
```bash
docker network create anchor-network
```

### 2. Start PostgreSQL
```bash
docker run -d \
  --name anchor-postgres \
  --network anchor-network \
  -e POSTGRES_USER=anchor \
  -e POSTGRES_PASSWORD=your_secure_password \
  -e POSTGRES_DB=anchor \
  -v anchor_postgres_data:/var/lib/postgresql/data \
  -p 5432:5432 \
  postgres:16-alpine
```

### 3. Wait for PostgreSQL to be Ready
```bash
docker exec anchor-postgres pg_isready -U anchor -d anchor
```

### 4. Build the Anchor Image
```bash
docker build -t anchor:latest .
```

### 5. Run the Anchor Application
```bash
docker run -d \
  --name anchor-app \
  --network anchor-network \
  -e POSTGRES_HOST=anchor-postgres \
  -e POSTGRES_PORT=5432 \
  -e POSTGRES_USER=anchor \
  -e POSTGRES_PASSWORD=your_secure_password \
  -e POSTGRES_DB=anchor \
  -e POSTGRES_SSLMODE=disable \
  -e ADMIN_JWT_SECRET=your_very_secure_jwt_secret_key_here \
  -e ENVIRONMENT=production \
  -e SERVER_PORT=8080 \
  -p 8080:8080 \
  anchor:latest
```

## Production Deployment

### Required Environment Variables

The following environment variables are **mandatory** and the application will fail to start without them:

- `POSTGRES_USER` - Database username
- `POSTGRES_PASSWORD` - Database password  
- `POSTGRES_DB` - Database name
- `ADMIN_JWT_SECRET` - Secret key for JWT token signing

### Optional Environment Variables

These have sensible defaults but can be customized:

- `ENVIRONMENT` (default: production)
- `SERVER_PORT` (default: 8080)
- `POSTGRES_HOST` (default: postgres)
- `POSTGRES_PORT` (default: 5432)
- `POSTGRES_SSLMODE` (default: disable)
- `REFRESH_TOKEN_LIFETIME` (default: 84600 seconds)
- `ACCESS_TOKEN_LIFETIME` (default: 3600 seconds)

### Security Considerations

1. **Use strong passwords:** Generate secure passwords for database and JWT secrets
2. **Enable SSL:** In production, set `POSTGRES_SSLMODE=require`
3. **Network isolation:** Use Docker networks to isolate services
4. **Regular updates:** Keep base images updated
5. **Secrets management:** Use Docker secrets or external secret management systems

### Example Production Docker Compose

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER_FILE: /run/secrets/postgres_user
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres_password
      POSTGRES_DB: anchor
    volumes:
      - postgres_data:/var/lib/postgresql/data
    networks:
      - anchor-internal
    secrets:
      - postgres_user
      - postgres_password

  anchor:
    image: anchor:latest
    restart: unless-stopped
    environment:
      POSTGRES_HOST: postgres
      POSTGRES_USER_FILE: /run/secrets/postgres_user
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres_password
      POSTGRES_DB: anchor
      POSTGRES_SSLMODE: require
      ADMIN_JWT_SECRET_FILE: /run/secrets/jwt_secret
    ports:
      - "8080:8080"
    networks:
      - anchor-internal
    depends_on:
      - postgres
    secrets:
      - postgres_user
      - postgres_password
      - jwt_secret

volumes:
  postgres_data:

networks:
  anchor-internal:
    driver: bridge

secrets:
  postgres_user:
    external: true
  postgres_password:
    external: true
  jwt_secret:
    external: true
```

## Monitoring and Maintenance

### Health Checks
- Application health: `curl http://localhost:8080/health`
- Database health: `docker exec anchor-postgres pg_isready -U anchor`

### Logs
```bash
# View application logs
docker-compose logs -f anchor

# View database logs
docker-compose logs -f postgres

# View logs for all services
docker-compose logs -f
```

### Backup
```bash
# Database backup
docker exec anchor-postgres pg_dump -U anchor anchor > backup.sql

# Restore from backup
docker exec -i anchor-postgres psql -U anchor anchor < backup.sql
```

### Updates
```bash
# Pull latest images
docker-compose pull

# Rebuild and restart
docker-compose up -d --build

# Remove old images
docker image prune
```

## Troubleshooting

### Common Issues

1. **Application fails to start:**
   - Check that all required environment variables are set
   - Verify database connectivity
   - Check logs: `docker-compose logs anchor`

2. **Database connection issues:**
   - Ensure PostgreSQL is healthy: `docker-compose ps`
   - Check network connectivity between containers
   - Verify database credentials

3. **Port conflicts:**
   - Change exposed ports in docker-compose.yml
   - Check what's running on ports: `netstat -tulpn | grep :8080`

### Debug Mode
```bash
# Run with debug output
docker-compose up

# Execute shell in running container
docker exec -it anchor-app sh

# Check container environment
docker exec anchor-app env
```

## Development

For development setup with hot reloading and development features, see the main README.md file.