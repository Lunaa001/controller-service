# NotebookUm Controller Service

**API Gateway y Orquestador** de la arquitectura de microservicios NotebookUm, desarrollado en **Go + Gin**.

## 📋 Descripción

El Controller es responsable de:
- ✅ Recibir todas las peticiones HTTP del cliente (Angular)
- ✅ Orquestar el flujo completo de procesamiento de documentos
- ✅ Coordinar llamadas entre microservicios (User, Extract, Summary, Persistence)
- ✅ Cachear resultados en Redis para optimizar performance
- ✅ Propagar IDs de correlación para trazabilidad
- ✅ Implementar circuit breakers para resiliencia
- ❌ NO accede directamente a la base de datos
- ❌ NO contiene lógica de negocio compleja

## 🏗️ Arquitectura

```
Frontend (Angular)
        ↓
   Controller (Go)
        ↓
   Redis (Cache)
        ↓
┌───────┴────────┬──────────────┬──────────────┐
│                │              │              │
User Service  Extract Service  Summary Service  Persistence (Java)
(Python)      (Python)         (Python)
```

## 🚀 Instalación y Configuración

### Requisitos Previos
- Go 1.26+
- Docker y Docker Compose (opcional)
- Redis (incluido en docker-compose)

### Instalación Local

1. **Clonar repositorio**
```bash
git clone <repository>
cd controller-service
```

2. **Instalar dependencias Go**
```bash
go mod download
go mod tidy
```

3. **Descargar módulos específicos (si es necesario)**
```bash
go get github.com/redis/go-redis/v9
go get github.com/gin-gonic/gin
go get github.com/google/uuid
```

4. **Configurar variables de entorno**
```bash
cp .env.example .env
# Editar .env con tus valores
```

5. **Ejecutar localmente**
```bash
go run ./cmd/controller
```

El servidor estará disponible en `http://localhost:5000`

## 🐳 Docker Compose

### Ejecución con Docker Compose

```bash
# Construir y ejecutar
docker compose up --build -d

# Ver logs
docker compose logs -f controller

# Detener
docker compose down
```

### Variables de Entorno en Docker Compose

Crear un archivo `.env` en la raíz del proyecto:

```env
REDIS_PASSWORD=tu_password_secreto
REDIS_URL=redis://redis:6379
USER_SERVICE_URL=http://user-service:8001
EXTRACT_SERVICE_URL=http://extract-service:8002
SUMMARY_SERVICE_URL=http://summary-service:8003
PERSISTENCE_URL=http://persistence-service:8004
```

## 📝 Endpoints

### Públicos (sin autenticación)

```
GET  /health                 → Health check básico
GET  /ready                  → Readiness check (verifica servicios)
GET  /status/circuits        → Estado de Circuit Breakers
```

### Autenticados (requieren X-User-ID o Bearer token)

```
POST   /api/v1/users                    → Crear usuario
GET    /api/v1/users/:id                → Obtener usuario
POST   /api/v1/documents/upload         → Subir y procesar PDF
GET    /api/v1/documents/:id            → Obtener estado del documento
GET    /api/v1/summaries/document/:id   → Obtener resumen
```

## 🔄 Flujo de Procesamiento

### Upload de Documento

```
1. Cliente envía PDF → POST /api/v1/documents/upload
2. Controller valida PDF
3. Extract Service: extrae texto del PDF
4. Persistence Service: guarda texto extraído
5. Summary Service: genera resumen con IA
6. Persistence Service: guarda resumen
7. Redis: cachea resultado (TTL: 24h)
8. Retorna respuesta al cliente
```

### Consulta de Documento

```
1. Cliente consulta → GET /api/v1/documents/:id
2. Controller consulta Redis (rápido)
3. Si no existe → consulta Persistence Service
4. Retorna resultado
```

## 🔒 Autenticación

El Controller soporta dos métodos de autenticación:

### 1. Header X-User-ID
```bash
curl -H "X-User-ID: user-123" http://localhost:5000/api/v1/users/user-123
```

### 2. Bearer Token (Authorization)
```bash
curl -H "Authorization: Bearer user-123" http://localhost:5000/api/v1/users/user-123
```

## 📊 Monitoreo

### Health Checks

```bash
# Health básico
curl http://localhost:5000/health

# Readiness (verifica dependencias)
curl http://localhost:5000/ready

# Estado de circuit breakers
curl http://localhost:5000/status/circuits
```

### Respuesta de Readiness
```json
{
  "status": "ok",
  "upstreams": [
    "redis",
    "user-service",
    "extract-service",
    "summary-service",
    "persistence-service"
  ]
}
```

## 🏗️ Estructura del Proyecto

```
controller-service/
├── cmd/
│   └── controller/
│       └── main.go                 # Entry point
├── internal/
│   ├── config/
│   │   └── config.go               # Configuración y variables de entorno
│   ├── core/
│   │   ├── cache/
│   │   │   └── redis.go            # Cliente Redis
│   │   ├── orchestrator/
│   │   │   └── saga.go             # Orquestador del flujo (Saga pattern)
│   │   └── resilience/
│   │       └── registry.go         # Circuit breaker registry
│   ├── transport/
│   │   ├── upstream/
│   │   │   └── client.go           # Cliente HTTP genérico
│   │   └── services/
│   │       ├── user.go             # Cliente User Service
│   │       ├── extract.go          # Cliente Extract Service
│   │       ├── summary.go          # Cliente Summary Service
│   │       └── persistence.go      # Cliente Persistence Service
│   └── web/
│       ├── handlers/
│       │   ├── health.go
│       │   ├── users.go
│       │   ├── documents.go        # [REESCRITO] Usa orquestador
│       │   └── summaries.go        # [REESCRITO] Usa orquestador
│       ├── middleware/
│       │   ├── auth.go
│       │   └── correlation.go
│       ├── problem/
│       │   └── problem.go          # RFC 9457 error responses
│       ├── validators/
│       │   └── validators.go
│       └── server.go               # [MODIFICADO] Inicializa servicios
├── go.mod                          # [ACTUALIZADO] Agregar redis
├── go.sum
├── Dockerfile
├── docker-compose.yml              # [ACTUALIZADO] Completo y funcional
├── .env.example                    # [ACTUALIZADO] Variables de entorno
└── README.md                       # Este archivo
```

## 🔧 Cambios Principales Realizados

### ✅ REUTILIZADO (78% del código)
- Estructura de carpetas (cmd/, internal/)
- Middleware de autenticación y correlation ID
- Cliente HTTP genérico (upstream.Client)
- Error handling (RFC 9457)
- Health checks
- Validadores

### 🔄 MODIFICADO (8%)
1. **config.go** - Agregar URLs de servicios y Redis
2. **registry.go** - Actualizar servicios a los reales
3. **server.go** - Inicializar todos los servicios y orquestador

### ❌ ELIMINADO (2%)
- `internal/domain/documents/store.go` - Store en RAM (reemplazado por Redis + Persistence)

### 🆕 CREADO (12%)
1. `internal/core/cache/redis.go` - Cliente Redis
2. `internal/core/orchestrator/saga.go` - Orquestador del flujo
3. `internal/transport/services/*.go` - Clientes de servicios
4. `internal/web/handlers/documents.go` - [REESCRITO]
5. `internal/web/handlers/summaries.go` - [REESCRITO]

## 📦 Dependencias Go

```bash
# Ver dependencias
go list -m all

# Ver qué usa cada dependencia
go mod graph

# Limpiar dependencias no usadas
go mod tidy
```

### Dependencias Principales
- `github.com/gin-gonic/gin` - Framework web
- `github.com/redis/go-redis/v9` - Cliente Redis
- `github.com/google/uuid` - Generación de UUIDs

## 🧪 Testing

```bash
# Ejecutar tests unitarios
go test ./...

# Con coverage
go test -cover ./...

# Tests específicos
go test -run TestDocumentsHandler ./internal/web/handlers
```

## 🔍 Debugging

### Logs
El servidor imprime logs automáticamente con Gin. Para más verbosidad:

```go
// En main.go
gin.SetMode(gin.DebugMode) // Default durante desarrollo
```

### Variables de Entorno para Debug
```bash
# Ver configuración cargada
LOG_LEVEL=debug go run ./cmd/controller
```

## 📱 Cliente de Ejemplo

### Subir y Procesar Documento

```bash
# Variables
USER_ID="user-123"
FILE="document.pdf"
HOST="localhost:5000"

# Upload
curl -X POST \
  -H "X-User-ID: $USER_ID" \
  -H "X-Correlation-ID: $(uuidgen)" \
  -F "file=@$FILE" \
  http://$HOST/api/v1/documents/upload

# Respuesta:
# {
#   "document_id": "550e8400-e29b-41d4-a716-446655440000",
#   "status": "ready",
#   "summary": "El documento contiene..."
# }
```

### Obtener Documento

```bash
DOC_ID="550e8400-e29b-41d4-a716-446655440000"

curl -H "X-User-ID: $USER_ID" \
  http://localhost:5000/api/v1/documents/$DOC_ID
```

## ⚙️ Configuración Avanzada

### Redis con Autenticación

```env
REDIS_URL=redis://redis:6379
REDIS_PASSWORD=my_secure_password
```

### Timeout de Requests

```env
REQUEST_TIMEOUT=30  # segundos
```

### URLs de Servicios (producción)

```env
USER_SERVICE_URL=https://user.prod.example.com
EXTRACT_SERVICE_URL=https://extract.prod.example.com
SUMMARY_SERVICE_URL=https://summary.prod.example.com
PERSISTENCE_URL=https://persistence.prod.example.com
```

## 🐛 Troubleshooting

### "Connection refused" a Redis
```bash
# Verificar que Redis está corriendo
docker ps | grep redis

# Verificar logs
docker logs notebookum-redis
```

### "Service unavailable" para User Service
```bash
# Verificar URL configurada
echo $USER_SERVICE_URL

# Probar conectividad
curl -i http://user-service:8001/health
```

### 502 Bad Gateway
- Verificar que todos los servicios están UP
- Ver `/status/circuits` para circuit breaker status
- Revisar logs del controller

## 📚 Referencias

- [Gin Framework](https://gin-gonic.com/)
- [Go Redis Client](https://redis.io/docs/clients/go/)
- [RFC 9457 - Problem Details](https://tools.ietf.org/html/rfc9457)
- [Saga Pattern](https://microservices.io/patterns/data/saga.html)
- [Circuit Breaker Pattern](https://martinfowler.com/bliki/CircuitBreaker.html)

## 👥 Contribuciones

Este Controller está diseñado para coordinarse con los siguientes servicios (desarrollados por otros equipos):
- **User Service** (Python)
- **Extract Service** (Python) 
- **Summary Service** (Python)
- **Persistence Service** (Java)

## 📄 Licencia

[Especificar licencia]
