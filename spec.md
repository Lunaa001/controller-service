# Especificación del Microservicio: Service-Controller-NotebookUm

**Versión**: 1.0  
**Fecha**: 2026-04-28  
**Estado**: Especificación  
**Rama de Funcionalidad**: 001-api-gestion-documentos  

---

## 1. Visión General

El **Service-Controller-NotebookUm** es el punto de entrada HTTP público del ecosistema de NotebookUm. Actúa como un enrutador inteligente e implementa el **patrón Strangler** para permitir la migración gradual de un monolito existente hacia una arquitectura de microservicios.

### Responsabilidades Principales

1. **Punto de Entrada Público**: Expone la API `/api/v1/` a clientes externos (web, móvil, terceros).
2. **Enrutamiento Inteligente**: Decide si cada petición debe procesarse por el monolito existente o delegarse a microservicios especializados (`service-user`, `service-persistence`, `service-extractor`, `service-ai`).
3. **Validación Coherente en el Borde**: Realiza validaciones de entrada (formato PDF, tamaño máximo 25MB, RFC 9457) sin duplicar lógica en otros servicios.
4. **Orquestación Mínima**: Compone llamadas a servicios cuando es necesario (ej.: verificar que el usuario existe antes de permitir una carga de documento).
5. **Seguridad y Autenticación**: Valida identidad de usuarios y autorización de acceso a recursos antes de delegar (coherente con el spec global).
6. **Uniformidad de Errores**: Garantiza que todas las respuestas de error sigan el formato RFC 9457.

### Fuera de Alcance

- Implementar el algoritmo de extracción de texto (Docling) — responsabilidad de `service-extractor`.
- Persistencia autoritativa de tablas (`usuarios`, `historial_documentos`, `historial_preguntas`, `resumenes`) — responsabilidad de `service-persistence`.
- Generación de resúmenes con LLM — responsabilidad de `service-ai`.
- Autenticación OAuth/JWT avanzada — se supone validación básica; ver **NEEDS CLARIFICATION** en la sección de Supuestos.

---

## 2. Historias de Usuario

### Historia 1: Enrutamiento Transparente de Solicitudes de Usuario (P1)

**Como** cliente de la API (web app, mobile app, terceros),  
**Quiero** que todas mis peticiones a `/api/v1/users` se ruteen correctamente al servicio responsable (monolito o `service-user`),  
**Para que** pueda crear mi cuenta y recuperar mis datos de forma confiable, sin preocuparme por la arquitectura interna.

#### Criterios de Aceptación

1. **Dado** que realizo un POST a `/api/v1/users` con datos de usuario válidos,  
   **Cuando** el controller recibe la solicitud,  
   **Entonces** valida los datos, los reenvía al servicio correcto (según configuración Strangler) y devuelve la respuesta con `id` de usuario generado (status 201).

2. **Dado** que realizo un GET a `/api/v1/users/{user_id}` con un ID válido,  
   **Cuando** el controller recibe la solicitud,  
   **Entonces** recupera los datos del usuario desde el servicio responsable y los devuelve (status 200).

3. **Dado** que intento crear un usuario con datos incompletos (ej.: falta email),  
   **Cuando** el controller valida la solicitud,  
   **Entonces** rechaza la petición con status 400 y respuesta RFC 9457 indicando qué campo falta.

4. **Dado** que realizo una solicitud a un endpoint inexistente `/api/v1/users/invalid`,  
   **Cuando** el controller procesa la solicitud,  
   **Entonces** devuelve status 404 con respuesta RFC 9457 coherente.

---

### Historia 2: Carga de Documento con Validación en el Borde (P1)

**Como** usuario autenticado,  
**Quiero** cargar un archivo PDF para procesamiento,  
**Para que** el sistema extraiga su contenido, genere un resumen y lo almacene asociado a mi cuenta.

#### Criterios de Aceptación

1. **Dado** que soy un usuario válido y cargo un PDF (≤25MB, `Content-Type: application/pdf`),  
   **Cuando** el controller recibe la solicitud POST a `/api/v1/documento/upload`,  
   **Entonces** valida el tipo de contenido y el tamaño en el borde, verifica que el usuario existe (llamando a `service-user` si es necesario), y delega el procesamiento (probablemente encolado o llamada asíncrona a `service-extractor`). Devuelve 202 (Accepted) con un `document_id`.

2. **Dado** que cargo un archivo que no es PDF (ej.: `.docx`, `.txt`),  
   **Cuando** el controller lo recibe,  
   **Entonces** rechaza la petición **en el borde** (antes de enviar a otros servicios) con status 400, respuesta RFC 9457 explicando que solo se aceptan PDFs.

3. **Dado** que cargo un archivo PDF que supera los 25MB,  
   **Cuando** el controller lo recibe,  
   **Entonces** rechaza la petición en el borde con status 400, respuesta RFC 9457 indicando el límite de tamaño. No se envía al servicio de extracción.

4. **Dado** que intento cargar un documento pero el usuario no existe (ID inválido),  
   **Cuando** el controller verifica con `service-user`,  
   **Entonces** devuelve status 404 o 401 según corresponda (usuario no encontrado o no autenticado), impidiendo la delegación al servicio de extracción.

5. **Dado** que la carga del documento es aceptada,  
   **Cuando** el servicio de extracción procesa el archivo,  
   **Entonces** el controller (potencialmente vía webhook o polling) eventualmente almacena los metadatos en `service-persistence` y devuelve el estado al cliente (según el flujo asíncrono definido).

---

### Historia 3: Consulta de Resumen con Control de Acceso (P1)

**Como** usuario autenticado,  
**Quiero** consultar el resumen de un documento que cargué,  
**Para que** pueda leer el contenido resumido sin procesar el PDF nuevamente.

#### Criterios de Aceptación

1. **Dado** que consulto GET `/api/v1/summaries/document/{document_id}` con un ID válido que me pertenece,  
   **Cuando** el controller recibe la solicitud,  
   **Entonces** verifica mi identidad y que soy el propietario del documento, delega a `service-persistence` para obtener el resumen, y devuelve el contenido con status 200.

2. **Dado** que intento consultar un resumen de un documento que pertenece a otro usuario,  
   **Cuando** el controller valida la autorización,  
   **Entonces** rechaza la petición con status 403 (Forbidden), sin revelar si el documento existe.

3. **Dado** que consulto un documento cuyo ID no existe,  
   **Cuando** el controller lo busca,  
   **Entonces** devuelve status 404 con respuesta RFC 9457.

4. **Dado** que consulto un documento cuyos resumen aún no está disponible (procesamiento en curso),  
   **Cuando** el controller lo verifica,  
   **Entonces** devuelve status 202 (Accepted) o 202 con un campo `status: "processing"` indicando que aún no está listo.

---

### Historia 4: Patrón Strangler - Coexistencia Monolito y Microservicios (P1)

**Como** arquitecto del sistema,  
**Quiero** poder configurar el controller para enrutar peticiones dinámicamente entre el monolito existente y microservicios,  
**Para que** podamos migrar gradualmente sin tiempo de inactividad.

#### Criterios de Aceptación

1. **Dado** que existe una configuración de Strangler (feature flags, mapeo de rutas),  
   **Cuando** el controller inicia,  
   **Entonces** carga la configuración desde variables de entorno o un servicio de configuración, permitiendo definir qué rutas van al monolito y cuáles a microservicios.

2. **Dado** que una petición llega al controller,  
   **Cuando** el controller evalúa la configuración Strangler,  
   **Entonces** enruta la solicitud al destino correcto (monolito vs microservicio) sin que el cliente lo note.

3. **Dado** que necesito cambiar el enrutamiento en tiempo de ejecución,  
   **Cuando** actualizo la configuración de Strangler (ej.: mover `GET /api/v1/users/{id}` del monolito a `service-user`),  
   **Entonces** el cambio se refleja en las siguientes peticiones sin reiniciar el controller (si la configuración soporta recarga dinámica).

4. **Dado** que un microservicio falla o está no disponible,  
   **Cuando** el controller intenta enrutar una petición a ese servicio,  
   **Entonces** implementa un fallback (ej.: reintentos, circuit breaker, o fallback al monolito si está disponible) y devuelve un estado coherente al cliente (status 503, RFC 9457).

5. **Dado** que necesito hacer rollback de una migración,  
   **Cuando** cambia la configuración Strangler,  
   **Entonces** las peticiones que fueron migrantes vuelven automáticamente al monolito sin pérdida de datos ni inconsistencias.

---

### Historia 5: Manejo de Microsservicios Caídos (P2)

**Como** cliente de la API,  
**Quiero** que el controller maneje gracefully los fallos de microservicios,  
**Para que** no vea errores oscuros internos y pueda saber si el problema es temporal o permanente.

#### Criterios de Aceptación

1. **Dado** que un microservicio (ej.: `service-user`) no responde dentro del timeout (ej.: 5s),  
   **Cuando** el controller intenta comunicarse,  
   **Entonces** registra el timeout en logs, devuelve status 503 (Service Unavailable) con respuesta RFC 9457 indicando que el servicio está temporalmente no disponible.

2. **Dado** que un microservicio devuelve un error 5xx,  
   **Cuando** el controller recibe esa respuesta,  
   **Entonces** no la pasa directamente al cliente. En su lugar, devuelve status 502 (Bad Gateway) o 503 con un mensaje genérico RFC 9457 (sin exponer detalles internos de error).

3. **Dado** que un microservicio está caído pero el monolito sigue operativo,  
   **Cuando** la configuración Strangler lo permite,  
   **Entonces** el controller intenta redirigir la petición al monolito como fallback.

4. **Dado** que ocurren múltiples fallos consecutivos al mismo servicio,  
   **Cuando** se abre un circuit breaker,  
   **Entonces** el controller rechaza rápidamente las siguientes peticiones a ese servicio sin esperar timeout, mejorando la respuesta general del sistema.

---

### Historia 6: Validación de Integridad entre Monolito y Microservicios (P2)

**Como** arquitecto del sistema,  
**Quiero** detectar inconsistencias cuando un cliente obtiene respuestas diferentes del monolito vs microservicios,  
**Para que** podamos identificar errores de migración tempranamente.

#### Criterios de Aceptación

1. **Dado** que existe un documento en el monolito pero no en `service-persistence`,  
   **Cuando** un cliente consulta el documento,  
   **Entonces** el controller registra la inconsistencia en logs (alertas), y devuelve una respuesta coherente al cliente (priorizando una fuente de verdad).

2. **Dado** que ambos sistemas devuelven respuestas para el mismo recurso,  
   **Cuando** sus contenidos difieren,  
   **Entonces** el controller implementa una política de resolución (ej.: preferir microservicio, preferir monolito, o devolver conflicto 409) documentada explícitamente en la configuración.

3. **Dado** que la configuración Strangler redirige a un nuevo servicio,  
   **Cuando** se inicia la migración,  
   **Entonces** se puede activar un modo "shadow" para escribir en ambos sistemas paralelamente y validar consistencia antes de cutover.

---

## 3. Requisitos Funcionales

### 3.1 Enrutamiento y Patrón Strangler

| ID | Requisito | Descripción |
|---|---|---|
| RF-CTRL-001 | Enrutamiento Configurable | El controller DEBE soportar configuración de rutas (Strangler) que defina si cada endpoint va al monolito o a microservicios. |
| RF-CTRL-002 | Feature Flags | El controller DEBE soportar feature flags para activar/desactivar rutas sin despliegue. |
| RF-CTRL-003 | Fallback Inteligente | Si un microservicio falla, el controller DEBE intentar fallback al monolito (si está configurado) o devolver error coherente. |
| RF-CTRL-004 | Circuit Breaker | El controller DEBE implementar circuit breaker para servicios con fallos repetidos. |
| RF-CTRL-005 | Timeout de Servicios | El controller DEBE establecer timeout (ej.: 5-30s según servicio) para evitar bloqueos prolongados. |

### 3.1.1 Separación CQRS en el Enrutamiento

El controller aplica separación command/query al nivel de enrutamiento, sin conocer la implementación interna de `service-persistence`. Esto permite que el persistence service escale lectura y escritura de forma independiente.

| Tipo | Endpoints | Destino |
|---|---|---|
| **command** | `POST /api/v1/documento/upload`, `POST /api/v1/summaries/{document_id}` | `service-persistence-write` (vía Saga Orchestrator) |
| **query** | `GET /api/v1/documents/{id}`, `GET /api/v1/summaries/{document_id}`, `GET /api/v1/history` | `service-persistence-read` |
| **legacy** | Cualquier endpoint no migrado aún | `monolito` |
| **internal** | `GET /health`, `GET /ready`, `GET /status/circuits` | Controller (no rutea a upstream) |

**Justificación**: el controller aplica esta separación en el ruteo sin necesidad de conocer si `service-persistence` implementa CQRS internamente o no. El beneficio es que la configuración Strangler puede apuntar commands y queries a destinos distintos de forma independiente.

### 3.2 Validación en el Borde (Edge Validation)

| ID | Requisito | Descripción |
|---|---|---|
| RF-CTRL-006 | Validación de Content-Type | El controller DEBE validar que archivos cargados a `/api/v1/documento/upload` tengan `Content-Type: application/pdf`. |
| RF-CTRL-007 | Validación de Tamaño | El controller DEBE rechazar archivos > 25MB con error 400 RFC 9457 **antes de enviar a otros servicios**. |
| RF-CTRL-008 | Validación de Estructura de Entrada | El controller DEBE validar que peticiones tengan estructura JSON válida (si aplica) antes de delegación. |
| RF-CTRL-009 | Validación de Identidad | El controller DEBE verificar que el usuario existe (consultando `service-user` o caché local) antes de permitir operaciones que requieran autenticación. |
| RF-CTRL-010 | Rate Limiting | El controller PUEDE implementar rate limiting por usuario/IP para prevenir abuso. |

### 3.3 Autenticación y Autorización

| ID | Requisito | Descripción |
|---|---|---|
| RF-CTRL-011 | Extracción de Identidad | El controller DEBE extraer la identidad del usuario desde headers (ej.: `Authorization: Bearer {token}`, o header personalizado si no hay JWT aún). |
| RF-CTRL-012 | Paso de Contexto | El controller DEBE propagar la identidad del usuario a los microservicios (ej.: agregando header `X-User-ID`). |
| RF-CTRL-013 | Aislamiento de Usuario | El controller DEBE garantizar que un usuario no pueda acceder a recursos de otro (verificación de propietario en GET /api/v1/summaries/document/{id}). |
| RF-CTRL-014 | Errores de Autenticación | El controller DEBE devolver status 401 (Unauthorized) si falta identidad o token inválido, con respuesta RFC 9457. |
| RF-CTRL-015 | Errores de Autorización | El controller DEBE devolver status 403 (Forbidden) si el usuario no tiene permisos para el recurso, con respuesta RFC 9457. |

### 3.4 Orquestación Mínima

| ID | Requisito | Descripción |
|---|---|---|
| RF-CTRL-016 | Verificación Previa | Antes de delegar POST /api/v1/documento/upload, el controller DEBE verificar que el usuario existe (llamada a `service-user`). |
| RF-CTRL-017 | Composición de Respuesta | El controller PUEDE componer respuestas combinando datos de múltiples servicios si el flujo lo requiere (ej.: usuario + historial). |
| RF-CTRL-018 | Encolado Asíncrono | Para operaciones de larga duración (ej.: procesamiento de PDF), el controller PUEDE encolar la tarea (ej.: Celery, RabbitMQ) y devolver 202 (Accepted) con ID de tarea. |
| RF-CTRL-019 | Polling de Estado | El controller PUEDE soportar un endpoint de polling (ej.: GET /api/v1/documents/{id}/status) para que clientes consulten el progreso. |

### 3.5 Manejo de Errores Uniforme

| ID | Requisito | Descripción |
|---|---|---|
| RF-CTRL-020 | RFC 9457 Compliance | El controller DEBE devolver todas las respuestas de error en formato RFC 9457 (Problem Details): `type`, `title`, `status`, `detail`, `instance`. |
| RF-CTRL-021 | Errores 4xx Originales | Si el microservicio devuelve un 4xx, el controller PUEDE reenviarlo, reformateándolo según RFC 9457 si es necesario. |
| RF-CTRL-022 | Errores 5xx Enmascarados | Si el microservicio devuelve un 5xx, el controller DEBE devolver 502/503 con mensaje genérico (sin exponer detalles internos). |
| RF-CTRL-023 | Errores de Timeout | Timeouts DEBEN tratarse como status 503 con RFC 9457 indicando "servicio no disponible temporalmente". |
| RF-CTRL-024 | Errores de Validación | Validaciones fallidas en el borde DEBEN devolver 400 con RFC 9457 detallando qué validó y por qué falló. |
| RF-CTRL-025 | Trazabilidad | Todo error DEBE incluir `instance` (ej.: `/api/v1/documento/upload?requestId=xyz`) para debugging. |

### 3.6 Seguridad de Entrada

| ID | Requisito | Descripción |
|---|---|---|
| RF-CTRL-026 | Sanitización de Parámetros | El controller DEBE sanitizar parámetros de URL y JSON para prevenir inyección (SQL, NoSQL, command injection). |
| RF-CTRL-027 | Límite de Payload | El controller DEBE rechazar peticiones con payload > 25MB (validación genérica antes de lectura). |
| RF-CTRL-028 | CORS Seguro | El controller DEBE implementar CORS con lista blanca de orígenes permitidos (configurable). |
| RF-CTRL-029 | Headers Seguros | El controller DEBE incluir headers de seguridad (ej.: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`). |

### 3.7 Logging, Monitoreo y Debugging

| ID | Requisito | Descripción |
|---|---|---|
| RF-CTRL-030 | Logging Estructurado | El controller DEBE registrar en logs (estructurados, JSON): petición recibida, enrutamiento, respuesta devuelta, tiempos. |
| RF-CTRL-031 | Métricas | El controller DEBE exponer métricas Prometheus (ej.: latencia, status codes, errores por servicio). |
| RF-CTRL-032 | Tracing Distribuido | El controller DEBE soportar tracing distribuido (ej.: OpenTelemetry, jaeger) para rastrear peticiones entre servicios. |
| RF-CTRL-033 | Logging de Errores | Errores internos (timeouts, circuit breaker abierto) DEBEN registrarse con contexto completo para debugging. |
| RF-CTRL-034 | Liveness Endpoint | El controller DEBE exponer `GET /health` → responde 200 `{"status":"ok"}` si el proceso está vivo. No verifica upstreams. |
| RF-CTRL-035 | Readiness Endpoint | El controller DEBE exponer `GET /ready` → responde 200 si todos los upstreams críticos responden, o 503 con detalle por upstream si alguno falla. |
| RF-CTRL-036 | Circuit Breaker Status | El controller DEBE exponer `GET /status/circuits` → array de `{service, state, failureCount}` para dashboards y Traefik. |

### 3.8 Patrones de Resiliencia

#### 3.8.1 Circuit Breaker por Upstream

El controller implementa un **Circuit Breaker independiente por cada upstream**: `service-user`, `service-persistence`, `service-extractor`, `service-ai` y `monolito`. Todos los umbrales son configurables vía variables de entorno.

| Estado | Descripción | Transición |
|---|---|---|
| **CLOSED** | Operación normal; las llamadas se realizan. | → OPEN al superar `CB_{SVC}_FAILURE_THRESHOLD` fallos consecutivos en `CB_{SVC}_WINDOW_SECONDS`. |
| **OPEN** | Circuito abierto; llamada rechazada inmediatamente con 503 RFC 9457. | → HALF-OPEN tras `CB_{SVC}_RECOVERY_TIMEOUT_SECONDS`. |
| **HALF-OPEN** | Se permite una petición de prueba al upstream. | → CLOSED si la prueba tiene éxito; → OPEN si falla. |

**Comportamiento en estado OPEN**: el controller devuelve `503 Service Unavailable` con cuerpo RFC 9457 sin propagar la llamada al upstream. El campo `detail` identifica el servicio con el circuito abierto.

**Parámetros configurables por upstream** (variables de entorno con prefijo `CB_{SERVICE}_`):
- `FAILURE_THRESHOLD` — fallos consecutivos para abrir el circuito (default: 5)
- `WINDOW_SECONDS` — ventana de observación de fallos (default: 60)
- `RECOVERY_TIMEOUT_SECONDS` — tiempo en OPEN antes de pasar a HALF-OPEN (default: 30)

**Observabilidad**: `GET /status/circuits` (RF-CTRL-036) expone el estado actual de cada breaker.

#### 3.8.2 Bulkhead por Upstream

Cada upstream dispone de un **pool de conexiones HTTP aislado** (no compartido entre servicios). Un semáforo por upstream limita la concurrencia máxima.

| Upstream | Variable de Entorno | Default |
|---|---|---|
| `service-user` | `BH_USER_MAX_CONCURRENT` | 20 |
| `service-persistence` | `BH_PERSISTENCE_MAX_CONCURRENT` | 30 |
| `service-extractor` | `BH_EXTRACTOR_MAX_CONCURRENT` | 10 |
| `service-ai` | `BH_AI_MAX_CONCURRENT` | 5 |
| `monolito` | `BH_MONOLITH_MAX_CONCURRENT` | 50 |

**Comportamiento en exceso de concurrencia**: las peticiones que superen el límite devuelven `503 Service Unavailable` con cuerpo RFC 9457 **sin encolarse**. No existe queue de espera.

**Justificación**: evitar que la lentitud de `service-ai` (latencias de decenas de segundos por llamada a LLM) agote los workers del controller e impida servir peticiones rápidas hacia `service-user`.

### 3.9 Rate Limiting

El controller aplica rate limiting en dos dimensiones:

| Dimensión | Identificador | Uso |
|---|---|---|
| Por usuario autenticado | `user_id` extraído del header `Authorization` | Límite principal |
| Por IP | IP de origen del cliente | Fallback cuando no hay autenticación |

**Límites diferenciados por tipo de operación**:

| Operación | Endpoints | Límite | Ventana |
|---|---|---|---|
| Uploads | `POST /api/v1/documento/upload` | 10 req | 1 min |
| Creación de recursos | `POST /api/v1/users`, `POST /api/v1/summaries/{id}` | 30 req | 1 min |
| Consultas | `GET *` | 100 req | 1 min |

**Respuesta en exceso**: `429 Too Many Requests` con cuerpo RFC 9457 y headers:
- `X-RateLimit-Limit`: límite configurado para el endpoint
- `X-RateLimit-Remaining`: peticiones restantes en la ventana actual
- `Retry-After`: segundos hasta el reset de la ventana

> **NEEDS CLARIFICATION**: ¿Los límites son fijos en configuración (variables de entorno) o dinámicos por plan de usuario (ej.: plan gratuito vs premium)? La arquitectura actual asume límites fijos configurables. Si se requieren límites dinámicos, se necesita integración adicional con `service-user` para consultar el plan del usuario en cada petición.

### 3.10 Saga de Upload

El flujo de carga de documento sigue el patrón **Saga Orchestrator** gestionado por el controller. Los pasos están ordenados y cada uno tiene una compensación definida.

#### Pasos de la Saga

| # | Step | Servicio | Compensación si falla |
|---|---|---|---|
| 1 | `validate-file` | Controller (edge) | — (falla inmediata 400; no hay estado que revertir) |
| 2 | `enqueue-extraction` | `service-extractor` | Cancelar tarea de extracción si fue encolada |
| 3 | `await-extraction` | `service-extractor` | Marcar documento como `FAILED`, notificar al cliente |
| 4 | `enqueue-ai-processing` | `service-ai` | Cancelar tarea de IA si fue encolada |
| 5 | `await-ai` | `service-ai` | Marcar documento como `FAILED`, notificar al cliente |
| 6 | `persist-result` | `service-persistence-write` | Rollback en persistence; reintentar hasta N veces |

#### Estados del Documento

| Estado | Descripción |
|---|---|
| `PENDING` | Documento recibido; en espera de inicio de extracción |
| `EXTRACTING` | Extracción de texto en curso (steps 2-3) |
| `PROCESSING` | Generación de resumen con IA en curso (steps 4-5) |
| `READY` | Saga completada; resumen disponible |
| `FAILED` | Saga fallida en algún step; causa registrada |

#### Compensaciones Clave

- Si `persist-result` falla → rollback en `service-persistence` (eliminar el registro parcial)
- Si `await-ai` falla → marcar documento como `FAILED` y notificar al cliente vía polling o webhook

> **NEEDS CLARIFICATION**: ¿La orquestación es **síncrona** (el controller mantiene la conexión abierta y espera cada step con timeouts individuales) o **asíncrona** (cola de mensajes tipo Celery/RabbitMQ donde el controller publica eventos y consume resultados)? Ambas opciones se dejan documentadas hasta que se cierre la decisión con el equipo de infraestructura:
>
> - **Opción A — Síncrona**: el controller encadena llamadas con timeouts configurables por step. Más simple de implementar, pero bloquea workers durante el procesamiento de IA (puede tardar varios minutos). Solo viable si el timeout total hacia el cliente es suficientemente largo.
>
> - **Opción B — Asíncrona** (recomendada para producción): el controller encola el trabajo, devuelve `202 Accepted` con `document_id`, y el cliente hace polling sobre `GET /api/v1/documents/{id}/status`. Requiere sistema de colas (Celery + Redis, o RabbitMQ).

---

## 4. Casos Extremos y Situaciones de Borde

### 4.1 Fallos de Microservicios

| Caso | Escenario | Comportamiento Esperado |
|---|---|---|
| MS-001 | `service-user` no responde al verificar usuario antes de carga de documento. | Controller devuelve 503 (Service Unavailable) RFC 9457, no delega la carga a `service-extractor`. |
| MS-002 | `service-extractor` está caído pero el cliente envía documento. | Controller acepta (202) si permite encolado, o devuelve 503 si requiere servicio activo. Configuración Strangler define comportamiento. |
| MS-003 | `service-persistence` falla después de que `service-extractor` procesa con éxito. | El controller intenta reintentos, usa cola de reprocesamiento, o registra incidencia. Transaccionalidad entre servicios es responsabilidad de orquestador superior (Saga pattern). |
| MS-004 | Respuesta corrupta desde microservicio (JSON inválido, timeout parcial). | Controller registra error, devuelve 502 RFC 9457 "Bad Gateway". |

### 4.2 Inconsistencias Monolito vs Microservicio

| Caso | Escenario | Comportamiento Esperado |
|---|---|---|
| INC-001 | Usuario existe en monolito pero no en `service-user` (durante migración). | Si ambos se consultan, el controller registra la inconsistencia. La configuración Strangler define fuente de verdad. |
| INC-002 | Documento está en monolito pero el resumen se busca en microservicio. | El controller intenta fallback al monolito si está configurado. Si no, devuelve 404. |
| INC-003 | Validación de archivo permite en monolito pero rechaza en controller (lógica desincronizada). | El controller debe tener su propia validación autoritativa en el borde (tamaño, tipo). |

### 4.3 Validación de Entrada Extrema

| Caso | Escenario | Comportamiento Esperado |
|---|---|---|
| VAL-001 | Cliente envía PDF de 24.9MB vs 25MB vs 25.1MB. | Primer y segundo acepto (202/201), tercero rechaza (400 RFC 9457) en el borde. |
| VAL-002 | Cliente envía JSON con campos extras no esperados. | Controller los ignora (no falla si son innecesarios) y delega. Si son críticos, valida en borde. |
| VAL-003 | Cliente envía `Content-Type: text/plain` pero el contenido es PDF. | Controller rechaza en borde por Content-Type incorrecto (400 RFC 9457). El cliente debe usar el header correcto. |
| VAL-004 | Cliente envía archivo vacío (0 bytes) con Content-Type correcto. | Controller rechaza como "archivo inválido" (400 RFC 9457) o delega a `service-extractor` que lo rechace. Comportamiento configurable. |

### 4.4 Acceso Cruzado de Usuarios

| Caso | Escenario | Comportamiento Esperado |
|---|---|---|
| SEC-001 | Usuario A intenta GET /api/v1/summaries/document/xyz que pertenece a Usuario B. | Controller valida propietario antes de delegar. Devuelve 403 Forbidden sin revelar si el documento existe. |
| SEC-002 | Usuario A intenta UPDATE/DELETE documento de Usuario B. | Controller rechaza con 403 Forbidden. No delega a servicio. |
| SEC-003 | Usuario no autenticado intenta cualquier operación. | Controller rechaza con 401 Unauthorized RFC 9457. No delega. |
| SEC-004 | Token expirado o inválido en Authorization header. | Controller rechaza con 401 RFC 9457. Si existe refresh token, puede ofrecer mecanismo de refresco. |

### 4.5 Concurrencia y Sobrecargas

| Caso | Escenario | Comportamiento Esperado |
|---|---|---|
| LOAD-001 | 100 clientes cargan documentos simultáneamente. | Controller distribuye encolamiento (si aplica) equitativamente. Rate limiting previene abuso. |
| LOAD-002 | Un cliente carga 50 documentos en 1 minuto. | Rate limiting o quota por usuario la detecta y rechaza con 429 (Too Many Requests) RFC 9457. |
| LOAD-003 | Microservicio alcanza su propio límite de concurrencia. | Devuelve 503 (Overloaded). Controller propaga al cliente como 503 RFC 9457 o intenta fallback. |

### 4.6 Recuperación de Fallos y Rollback

| Caso | Escenario | Comportamiento Esperado |
|---|---|---|
| REC-001 | Un microservicio se cae, se migra un endpoint desde monolito a microservicio. | Si microservicio se recupera después, nuevas peticiones van a él. Datos históricos permanecen (sin pérdida). |
| REC-002 | Migración incompleta: parte de la lógica cambió, pero logs no. | Controller mantiene coherencia mediante Strangler configuration. Si hay inconsistencia, se detecta y registra. |
| REC-003 | Necesidad de rollback: revertir endpoint de microservicio a monolito. | Configuración Strangler cambia. Peticiones fluyen de nuevo al monolito. Sin tiempo de inactividad si rollback es limpio. |

### 4.7 Resúmenes y Procesamiento Asíncrono

| Caso | Escenario | Comportamiento Esperado |
|---|---|---|
| ASYNC-001 | Cliente consulta GET /api/v1/summaries/document/xyz antes de que termine procesamiento. | Controller devuelve 202 Accepted o custom status indicando "procesamiento en progreso". |
| ASYNC-002 | Procesamiento falla en `service-extractor`. El controller intenta reintentos. | Tras N reintentos, marca documento como "error en procesamiento" y notifica al cliente (polling o webhook). |
| ASYNC-003 | Cliente intenta descargar documento antes de procesamiento completarse. | Si endpoint existe, el controller devuelve 202 (no listo) o 404 según flujo. |

---

## 5. Requisitos No Funcionales

### 5.1 Performance y Escalabilidad

| ID | Requisito | Descripción |
|---|---|---|
| RNF-001 | Latencia de Validación | Validaciones en el borde (tamaño, Content-Type) DEBEN completarse en < 100ms. |
| RNF-002 | Latencia de Enrutamiento | Decisión de enrutamiento (Strangler config) DEBE completarse en < 50ms. |
| RNF-003 | Escalabilidad Horizontal | El controller DEBE soportar desplegarse en múltiples instancias sin estado (stateless) o con estado compartido (Redis, caché distribuida). |
| RNF-004 | Throughput Mínimo | El controller DEBE soportar al menos 100 RPS (requests per second) sin degradación > 10%. |
| RNF-005 | Timeout de Servicios | Timeout máximo a microservicios: 30s (configurable por servicio). Timeout total a cliente: 60s. |

### 5.2 Confiabilidad

| ID | Requisito | Descripción |
|---|---|---|
| RNF-006 | Disponibilidad | El controller DEBE tener disponibilidad objetivo de 99.5% (mensual). |
| RNF-007 | Circuit Breaker | Circuit breaker DEBE abrirse tras 5 fallos consecutivos, y reintentar después de 30s (configurable). |
| RNF-008 | Reintentos | Reintentos automáticos para errores transitorios (5xx) hasta 3 veces con backoff exponencial. |
| RNF-009 | Healthcheck | El controller DEBE exponer endpoint `/health` que devuelva estado de servicios dependientes. |

### 5.3 Seguridad

| ID | Requisito | Descripción |
|---|---|---|
| RNF-010 | Validación de Entrada | 100% de inputs validados antes de procesamiento. |
| RNF-011 | HTTPS | El controller DEBE recibir/enviar tráfico HTTPS (TLS 1.2+). |
| RNF-012 | Secrets Management | Tokens, claves de API DEBEN almacenarse en variables de entorno o secrets manager, no hardcodeadas. |
| RNF-013 | Auditoría | Acceso a recursos sensibles DEBE registrarse en logs de auditoría (ej.: consulta a datos de otro usuario rechazada). |

### 5.4 Observabilidad

| ID | Requisito | Descripción |
|---|---|---|
| RNF-014 | Logs Estructurados | Todos los logs DEBEN ser JSON estructurados con campos: timestamp, level, service, message, user_id, request_id, status_code. |
| RNF-015 | Request ID | Cada petición DEBE tener un ID único (generado por controller, propagado a microservicios) para tracing. |
| RNF-016 | Métricas Prometheus | Métricas en `/metrics`: latencia (histogram), status codes (counter), errores por tipo (counter). |
| RNF-017 | Tracing Distribuido | Soporte para OpenTelemetry: spans para cada servicio llamado. |

### 5.5 Trazabilidad y Propagación de Contexto

| ID | Requisito | Descripción |
|---|---|---|
| RNF-018 | Correlation ID — Generación | El controller DEBE generar un `X-Correlation-ID` (UUID v4) en cada request entrante que no lo incluya en el header. |
| RNF-019 | Correlation ID — Propagación | El `X-Correlation-ID` DEBE propagarse en **todas** las llamadas salientes a upstreams (service-user, service-persistence, service-extractor, service-ai, monolito). |
| RNF-020 | Correlation ID — Respuesta | El `X-Correlation-ID` DEBE incluirse en el header de la respuesta al cliente para permitir trazabilidad end-to-end. |
| RNF-021 | Correlation ID — Logs | Todos los registros de log generados durante el procesamiento de una petición DEBEN incluir el `X-Correlation-ID` en el campo `correlation_id`. |

> **Nota**: el Correlation ID era previamente conocido como "Request ID" en plan.md. Se unifica aquí como `X-Correlation-ID` (estándar de facto en ecosistemas de microservicios) y se eleva a requisito no funcional autoritativo.

---

## 6. Arquitectura y Componentes

### 6.1 Flujo General de Petición

```
┌─────────────┐
│ Cliente HTTP│
└──────┬──────┘
       │ POST /api/v1/documento/upload (PDF + user_id)
       ▼
┌──────────────────────────────────────────────┐
│      Service-Controller-NotebookUm           │
├──────────────────────────────────────────────┤
│ 1. Extrae identidad del usuario (header)     │
│ 2. Valida Content-Type (application/pdf)     │
│ 3. Valida tamaño (≤25MB)                     │
│ 4. Verifica usuario existe (llamada a S-User)│
│ 5. Evalúa configuración Strangler            │
│ 6. Decide destino: monolito o S-Extractor   │
│ 7. Delega (encolando o llamada síncrona)    │
│ 8. Devuelve 202 + document_id al cliente    │
└──────────────────────────────────────────────┘
       │
       ├──────────────────┬────────────────┐
       │                  │                │
       ▼                  ▼                ▼
   ┌─────────┐      ┌──────────┐     ┌─────────────┐
   │ Monolito│      │S-Extractor│    │Service-User │
   └─────────┘      └──────────┘     └─────────────┘
                          │
                          ▼
                    ┌──────────────┐
                    │Service-AI    │
                    │(summarize)   │
                    └──────────────┘
                          │
                          ▼
                    ┌──────────────┐
                    │Service-       │
                    │Persistence   │
                    └──────────────┘
```

### 6.2 Componentes Internos del Controller

1. **HTTP Handler / Router**
   - Recibe peticiones HTTP
   - Mapea rutas a handlers internos
   - Devuelve respuestas HTTP

2. **Auth & Identity Extractor**
   - Extrae token/identidad del header Authorization
   - Valida formato (Bearer token, custom header, etc.)
   - Propaga identidad a microservicios como `X-User-ID`

3. **Edge Validator**
   - Valida Content-Type (PDF)
   - Valida tamaño de archivo (≤25MB)
   - Valida estructura JSON
   - Devuelve errores RFC 9457 inmediatamente si falla

4. **Strangler Configuration Manager**
   - Carga configuración de rutas (archivo, variables de entorno, servicio central)
   - Permite feature flags (ruta A → monolito vs ruta A → microservicio)
   - Soporta recarga dinámica (sin reinicio)

5. **Router / Orchestrator**
   - Evalúa configuración Strangler
   - Decide destino de cada petición
   - Compone llamadas si es necesario (ej.: verificar usuario antes de procesar)
   - Maneja encolado asíncrono

6. **Circuit Breaker & Resilience Manager**
   - Monitorea fallos de servicios
   - Abre/cierra circuitos
   - Implementa reintentos con backoff
   - Fallback a monolito si está configurado

7. **Error Handler**
   - Normaliza errores de microservicios
   - Formatea respuestas RFC 9457
   - Enmascara errores 5xx internos

8. **Logger & Metrics**
   - Logs estructurados (JSON)
   - Métricas Prometheus
   - Tracing distribuido (OpenTelemetry)

---

## 7. Supuestos y Restricciones

### 7.1 Supuestos Explícitos

| ID | Supuesto | Impacto | Estado |
|---|---|---|---|
| SUP-001 | Los microservicios exponen APIs REST (no RPC, GraphQL). | El controller implementa cliente HTTP. Si cambian, adaptador necesario. | Asumido |
| SUP-002 | Existe un mecanismo de autenticación de usuario (token en header, sesión, etc.). | El controller extrae identidad. Detalles en NEEDS CLARIFICATION. | **NEEDS CLARIFICATION** |
| SUP-003 | El monolito existente sigue disponible durante la migración. | Patrón Strangler es viable. Si se detiene, requiere migración total. | Asumido |
| SUP-004 | Los microservicios (`service-user`, `service-persistence`, etc.) están disponibles y implementan endpoints esperados. | Si faltan, requiere fallback o desarrollo. | Asumido |
| SUP-005 | Documentos PDF contienen texto extraíble (no imágenes escaneadas sin OCR). | Si fallan, devuelven error, no corrupción silenciosa. | Asumido |
| SUP-006 | El servicio de generación de resúmenes (`service-ai`) soporta español e inglés. | Controller no valida idioma, delega. Si no soporta, error del servicio. | Asumido |
| SUP-007 | La persistencia autoritativa está en `service-persistence` (o monolito durante migración). | Controller no es fuente de verdad. | Asumido |
| SUP-008 | Los usuarios solo pueden acceder a sus propios documentos. | No existe compartir documentos en v1. | Asumido |
| SUP-009 | El encolado asíncrono (si se implementa) usa sistema confiable (ej.: Celery, RabbitMQ). | Tareas encoladas no se pierden. | Asumido |
| SUP-010 | Tiempos de procesamiento de documentos típicos son < 5 minutos. | Polling o webhook es viable para esperar resultado. | Asumido |

### 7.2 NEEDS CLARIFICATION

Los siguientes puntos requieren decisión antes del desarrollo:

1. **Autenticación de Usuario**
   - ¿Qué mecanismo se usa? (JWT, OAuth 2.0, sesión, API key personalizada)
   - ¿Dónde se valida el token? (¿En el controller o delegado a `service-user`?)
   - ¿Qué claims/información incluye el token?
   - ¿Cómo maneja tokens expirados? (rechazo vs refresh token)

2. **Encolado Asíncrono**
   - ¿Se usa Celery, RabbitMQ, AWS SQS, o algo else?
   - ¿El controller solo encola y devuelve 202, o verifica estado antes?
   - ¿Existe webhook para notificar resultado, o cliente hace polling?

3. **Versioning de API**
   - ¿v1 es la única versión activa, o se soportan múltiples versiones?
   - ¿Cómo se manejan cambios breaking? (versionado en URL, header, etc.)

4. **Configuración Strangler**
   - ¿Fuente de configuración? (archivo YAML, variables de entorno, etcd, Consul)
   - ¿Soporte para recarga dinámica (hot reload)?
   - ¿Política de resolución de conflictos si ambos (monolito + MS) devuelven respuestas?

5. **Límites y Cuotas**
   - ¿Hay rate limit por usuario? (ej.: 100 RPS por usuario, 1000 carga de documentos/mes)
   - ¿Cuál es el timeout máximo a microservicios? (actual: 30s, ¿es aceptable?)

6. **Auditoría y Compliance**
   - ¿Qué eventos se auditan? (acceso a documentos, cambios de usuario, etc.)
   - ¿Retención de logs? (ej.: 90 días)

7. **Recuperación en Caso de Error**
   - Si una carga de documento falla parcialmente (PDF procesado pero resumen fallido), ¿qué sucede?
   - ¿Reintentos automáticos, o esperar a usuario retry?

---

## 8. Criterios de Éxito

| ID | Criterio | Medida | Target |
|---|---|---|---|
| CS-001 | Tiempo de validación en borde | Latencia P95 de validación (Content-Type, tamaño) | < 100ms |
| CS-002 | Tasa de rechazo correcto | % de archivos no-PDF rechazados en el borde | 100% |
| CS-003 | Tasa de rechazo de archivos > 25MB | % de archivos > 25MB rechazados sin delegación | 100% |
| CS-004 | Enrutamiento correcto | % de peticiones enrutadas al destino correcto (Strangler config) | 100% |
| CS-005 | Aislamiento de usuario | % de intentos de acceso cruzado detectados y rechazados | 100% |
| CS-006 | Disponibilidad del controller | Uptime mensual | 99.5% |
| CS-007 | Uniformidad de errores | % de errores devueltos en RFC 9457 | 100% |
| CS-008 | Manejo de fallos de MS | Tiempo de detección de MS caído + fallback | < 10s (timeout + switch) |
| CS-009 | Trazabilidad | % de peticiones con request_id único | 100% |
| CS-010 | Migración sin downtime | Flujo endpoint: monolito → MS → monolito (rollback) sin pérdida de datos | 0 registros perdidos |

---

## 9. Integración con Otros Servicios

### 9.1 Service-User

- **Responsabilidad**: CRUD de usuarios, autenticación (delegada o validación)
- **Puntos de Contacto**:
  - POST `/api/v1/users` → delega a service-user (o monolito)
  - GET `/api/v1/users/{id}` → delega a service-user (o monolito)
  - Verificación de usuario antes de carga de documento
- **Contrato**: Devuelve user_id, metadata básico, status code HTTP

### 9.2 Service-Extractor

- **Responsabilidad**: Extracción de texto desde PDF (Docling)
- **Puntos de Contacto**:
  - POST `/api/v1/documento/upload` → delega a service-extractor (asíncrono)
- **Contrato**: Acepta archivo binary, devuelve document_id + estado (202 Accepted)

### 9.3 Service-AI

- **Responsabilidad**: Generación de resúmenes (LLM)
- **Puntos de Contacto**:
  - Llamado por service-extractor o service-persistence (no directo desde controller)
- **Contrato**: Acepta texto extraído, devuelve resumen

### 9.4 Service-Persistence

- **Responsabilidad**: Persistencia autoritativa (tablas usuarios, historial_documentos, historiales_preguntas, resumenes)
- **Puntos de Contacto**:
  - GET `/api/v1/summaries/document/{id}` → delega a service-persistence
  - Consultas de historial (potencialmente)
- **Contrato**: API CRUD, devuelve datos + metadata

### 9.5 Monolito Existente

- **Responsabilidad**: Lógica heredada (durante migración)
- **Puntos de Contacto**:
  - Cualquier endpoint no migrado aún
- **Contrato**: Same as above (HTTP REST)
- **Strangler**: Controller decide qué va aquí vs microservicios

---

## 10. Notas de Implementación

### 10.1 Stack Tecnológico Recomendado

- **Framework**: FastAPI + uvicorn/Granian (ASGI async — reemplaza Flask)
- **HTTP Client**: `httpx` async (reemplaza `requests` síncrono)
- **Circuit Breaker**: `pybreaker` (un breaker por upstream)
- **Rate Limiting**: `slowapi` (integración nativa FastAPI, basado en limits)
- **Bulkhead**: semáforos `anyio` (un semáforo por upstream)
- **Logging**: `structlog` o `python-json-logger`
- **Métricas**: `prometheus_client`
- **Tracing**: `opentelemetry`
- **Encolado asíncrono** (si Opción B Saga): Celery + Redis o RabbitMQ

### 10.2 Estructura de Código Sugerida

```
app/
├── __init__.py
├── config.py                    # Configuración Strangler
├── routes/
│   ├── __init__.py
│   ├── users.py                 # POST/GET /api/v1/users
│   ├── documents.py             # POST /api/v1/documento/upload
│   └── summaries.py             # GET /api/v1/summaries/document/{id}
├── middleware/
│   ├── auth.py                  # Extracción de identidad
│   ├── validation.py            # Edge validation
│   └── error_handler.py          # RFC 9457 formatting
├── services/
│   ├── strangler.py             # Lógica de enrutamiento
│   ├── circuit_breaker.py       # Resilience
│   └── orchestrator.py          # Composición de llamadas
├── clients/
│   ├── user_client.py           # Cliente a service-user
│   ├── persistence_client.py    # Cliente a service-persistence
│   ├── extractor_client.py      # Cliente a service-extractor
│   └── monolith_client.py       # Cliente a monolito
└── utils/
    ├── logger.py                # Logs estructurados
    └── metrics.py               # Prometheus metrics
```

### 10.3 Testing

- **Unit Tests**: Validación en borde, error handling, circuit breaker
- **Integration Tests**: Enrutamiento Strangler con servicios mockeds
- **Contract Tests**: Controller vs microservicios (validar API contracts)
- **Load Tests**: Validar throughput mínimo (100 RPS)

---

## 11. Plan de Migración (Strangler Pattern)

### Fase 1: Inicial (Todos endpoints al monolito)

```yaml
strangler:
  rules:
    - route: "POST /api/v1/users"
      destination: "monolith"
    - route: "GET /api/v1/users/:id"
      destination: "monolith"
    - route: "POST /api/v1/documento/upload"
      destination: "monolith"
    - route: "GET /api/v1/summaries/document/:id"
      destination: "monolith"
```

### Fase 2: Migración Gradual

```yaml
strangler:
  rules:
    - route: "POST /api/v1/users"
      destination: "service-user"  # Migrado
    - route: "GET /api/v1/users/:id"
      destination: "service-user"  # Migrado
    - route: "POST /api/v1/documento/upload"
      destination: "service-extractor"  # Migrado (asíncrono)
    - route: "GET /api/v1/summaries/document/:id"
      destination: "service-persistence"  # Migrado
```

### Fase 3: Validación de Consistencia

Durante la migración, se puede ejecutar en "shadow mode":
- Escribir en ambos sistemas (monolito + MS) en paralelo
- Validar respuestas son idénticas
- Detectar inconsistencias antes de cutover

### Fase 4: Rollback

Si problemas se detectan:
```yaml
strangler:
  rules:
    - route: "POST /api/v1/documento/upload"
      destination: "monolith"  # Revertir a monolito
```

---

## 12. Preguntas Abiertas (Decisions Pending)

1. ¿Cuál es el mecanismo de autenticación? (JWT, sesión, API key)
2. ¿Existe servicio de configuración centralizado para Strangler, o archivo local?
3. ¿Encolado asíncrono es obligatorio o opcional?
4. ¿Cuáles son los SLAs esperados para cada endpoint?
5. ¿Existe GDPR/compliance requirement para auditoría de acceso?
6. ¿Qué herramientas de observabilidad están disponibles? (Datadog, Prometheus, ELK)
7. ¿Endpoint de salud (`/health`) debe verificar estado de TODOS los microservicios o solo del controller?

---

## Appendix A: Formato RFC 9457 (Problem Details)

```json
{
  "type": "https://notebookum.example/errors/file-too-large",
  "title": "File Too Large",
  "status": 400,
  "detail": "The uploaded file exceeds the maximum size of 25MB. File size: 26MB.",
  "instance": "/api/v1/documento/upload?requestId=abc123xyz"
}
```

**Campos**:
- `type`: URI identificando la clase de error (reutilizable)
- `title`: Resumen legible
- `status`: HTTP status code
- `detail`: Explicación específica del incidente
- `instance`: referencia a la solicitud (para debugging)

---

## Appendix B: Glosario

| Término | Definición |
|---|---|
| **Strangler Pattern** | Patrón arquitectónico para migrar monolito a microservicios reemplazando gradualmente funcionalidad sin downtime. |
| **Edge Validation** | Validaciones realizadas en el punto de entrada (controller) antes de delegar a otros servicios. |
| **Circuit Breaker** | Patrón de resiliencia que abre un circuito después de N fallos consecutivos, rechazando peticiones rápidamente sin esperar timeout. |
| **RFC 9457** | Estándar IETF para formato uniforme de respuestas de error HTTP (Problem Details). |
| **Fallback** | Estrategia de contingencia (ej.: si MS falla, usar monolito como respaldo). |
| **Request ID** | Identificador único por petición, propagado entre servicios para tracing distribuido. |
| **Shadow Mode** | Ejecución paralela de ambos sistemas (monolito + MS) escribiendo en ambos para validar consistencia. |

---

**Fin de Especificación**

