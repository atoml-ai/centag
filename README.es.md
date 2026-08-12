# Centag

<p align="center">
  <strong>Tu hub de proxy LLM — el pipeline es la estrategia</strong><br/>
  Una pasarela proxy universal para grandes modelos. Gestiona de forma unificada todos los proveedores de backend, pools de API keys y estrategias de proxy personalizadas; define el comportamiento del Agent cliente con pipelines configurables y una arquitectura de plugins abierta.<br/>
  <em>Puede funcionar como un relay, pero es mucho más que un relay.</em>
</p>

<p align="center">
  <a href="https://github.com/atoml-ai/centag/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="License" /></a>
  <img src="https://img.shields.io/badge/go-1.25+-00ADD8?logo=go" alt="Go Version" />
  <a href="https://github.com/atoml-ai/centag/releases"><img src="https://img.shields.io/github/v/release/atoml-ai/centag" alt="Release" /></a>
  <a href="https://github.com/atoml-ai/centag/releases"><img src="https://img.shields.io/github/downloads/atoml-ai/centag/total" alt="Downloads" /></a>
</p>

<p align="center">
  <a href="README.md">English</a> | <a href="README.zh-CN.md">简体中文</a> | <a href="README.ja.md">日本語</a> | <a href="README.ko.md">한국어</a> | <a href="README.ru.md">Русский</a> | Español
</p>

---

## El problema que resolvemos

Un «relay» LLM típico solo reenvía la petición tal cual. Si cae una key, la cambias a mano; si el modelo no encaja, reconfiguras; cada Agent nuevo es otra ronda de ajustes. La estrategia vive en cada herramienta; la pasarela no tiene estrategia.

**Centag no es solo un relay: es un hub de proxy orquestable.** Pools de backends, failover y degradación, enrutado por escenario y medición/facturación convergen en un mismo pipeline; el Agent casi no se entera.

| Capacidad | Qué obtienes |
|---|---|
| **Gestión de pool de backends LLM** | OpenAI, Anthropic, Zhipu, Ollama y cualquier endpoint compatible en un solo sitio; varias keys y backends en la Web UI |
| **Failover · matching · degradación** | Rotación de keys ante rate limit; cambio de backend ante fallo; mejor egress por capacidad de modelo y carga |
| **Enrutado de modelos** | Cambia de modelo de backend en tiempo real según el tipo de pregunta — incluso en la misma sesión y tarea — sin reconfigurar el cliente |
| **Cambio de escenario del Agent** | Coding, Q&A, etc.: cada escenario su pipeline — cambiar escenario = cambiar estrategia, el Agent no lo nota |
| **Conexión rápida de Agents** | Escritura de config en un clic para Agents habituales; o proxy de proceso `centag wrap` sin cambios; guías en la Web UI si aún no hay one-click. La lista de soporte crece |
| **Estrategia de System Prompt** | Passthrough, append o replace del system prompt del cliente — conservar la persona del Agent, superponer normas o forzar un prompt unificado a nivel de pipeline |
| **Medición y facturación** | Tokens y coste por petición, backend y modelo |
| **Acceso de alto rendimiento sin pérdida** | Forward transparente y SSE passthrough — compatible de protocolo, bajo overhead, mínima reescritura de la semántica upstream |

---

## Ventajas clave

### Orquestación visual de pipelines

Un relay solo reenvía. **Centag te deja diseñar el ciclo de vida completo de la petición** — un DAG en el lienzo con drag-and-drop; el pipeline *es* la estrategia.

**16 tipos de nodos integrados**, combinables a voluntad:

| Nodo | Kind | Función |
|------|------|---------|
| Generator | `llm.generate` | Llamar a cualquier backend LLM para generar |
| Router | `route.decide` | Ramificar por intención, palabra clave o clasificación LLM |
| Scheduler | `scheduling.decide` | Programación y matching inteligente entre backends |
| Transparent Forward | `proxy.transparent_forward` | Proxy HTTP crudo (SSE passthrough) |
| Aggregator | `aggregate.merge` | Fusionar / votar / elegir lo mejor de generadores en paralelo |
| Reviewer | `quality.review` | Puntuar y auditar respuestas upstream |
| Memory | `memory.query` | Recuperar contexto de memoria cloud / vectores locales |
| Audit | `audit.safety` | Moderación y filtros de seguridad |
| Token Usage | `metrics.token_usage` | Seguimiento de tokens y coste |
| Cache | `cache.access` | Caché lectura/escritura (exacta / semántica / híbrida) |
| Processor | `content.transform` | Transformación y postproceso de contenido |
| Tool Call | `inject.tool_call` | Inyectar herramientas Function Calling |
| Prompt Ops | `prompt.ops` | Preproceso del user prompt |
| Output Post-ops | `prompt.postprocess` | Postproceso de la salida |
| Loop Controller | — | Control de bucles para workflows iterativos |
| Plugin Node | *(remoto / negocio)* | Nodos propios vía HTTP o Go SDK |

**Pipeline = estrategia.** Cambiar escenario → cambiar pipeline → el Agent no cambia ni una línea.

| Escenario | Ejemplo de pipeline |
|-----------|---------------------|
| Asistente de código | Router → modelo especializado → code review |
| Scheduling inteligente | Intención → matching por capacidad → failover |
| Compliance empresarial | Safety → generate → PII redact → audit |
| Soporte / RAG | Memoria o retrieval → generate → quality review |

### Backends unificados y pools de keys

| Capacidad | Detalle |
|-----------|---------|
| **Multi-backend** | Proveedores principales y endpoints compatibles con OpenAI, en una Web UI |
| **Pool de API keys** | Varias keys por backend; rotación automática ante límites o caídas |
| **Failover y degradación** | Key falla → siguiente; backend falla → siguiente |
| **Matching inteligente** | Pesos, prioridades y capacidad de modelo para el mejor egress |
| **Seguimiento de coste** | Tokens y dinero por petición, backend y modelo |

### Conexión rápida de Agents — tres vías

Conecta un Agent a Centag sin tocar el código de negocio. Elige según el nivel de adaptación:

| Método | Ideal para | Detalle |
|--------|------------|---------|
| **Escritura de config en un clic** | Agents habituales ya adaptados | La Web UI escribe Base URL / API Key, listo para usar |
| **Proxy de proceso centag wrap** | Cero cambios de config | Proxy transparente a nivel de proceso; el tráfico va a Centag sin tocar config ni código del Agent |
| **Guía en la UI** | Agents aún sin one-click | Pasos en la página para apuntar manualmente a la pasarela |

La lista de Agents habituales sigue creciendo; el resto puede usar la guía o wrap.

```bash
# Arrancar Centag
centag

# Ejemplo wrap — sin cambiar la config del Agent
centag wrap run -- opencode

# Autocomprobación
centag wrap doctor
```

### Ecosistema abierto de plugins

Los nodos del pipeline son extensibles: plugins locales con Go SDK, o plugins HTTP remotos en cualquier lenguaje.

```go
type NodePlugin interface {
    Descriptor() NodePluginDescriptor
    ValidateConfig(config NodeConfig) error
    Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error)
}
```

Contrato del plugin remoto:

```
GET  /.well-known/centag-node-plugin.json   →  auto-descubrimiento
POST /validate                               →  validación de config
POST /execute                                →  ejecutar el nodo
```

---

## Inicio rápido

```bash
# 1. Instalar (elige uno)
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
# o
npm install -g @atomlai/centag

# 2. Arrancar
centag

# 3. Web UI → http://localhost:20060 → añadir el primer backend

# 4. Conectar un Agent (config en un clic, o wrap sin cambios)
centag wrap run -- opencode
```

Listo. El tráfico pasa por Centag: pools de backends compartidos, failover, enrutado de modelos, visibilidad de costes.

> **Credenciales por defecto:** usuario `admin` — establece tu contraseña en el asistente de primera ejecución (sin contraseña predefinida). Opcionalmente predefinela con `LLM_PROXY_ADMIN_PASSWORD` antes del primer arranque.

### Otros métodos de instalación

<details>
<summary>npm (sin cambiar rutas globales)</summary>

```bash
npx --yes @atomlai/centag
```
</details>

<details>
<summary>Offline / red cerrada</summary>

```bash
npm install -g @atomlai/centag-offline
```
</details>

<details>
<summary>Docker (desde el código fuente)</summary>

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # edita secretos si hace falta
./start.sh docker build personal                     # construir imagen
./start.sh docker up personal                        # iniciar contenedor
```

UI de administración: http://localhost:20060 · Detener: `./start.sh docker down`

Los datos persistentes se almacenan en `deploy/docker/data/` (se crea automáticamente en el primer inicio).

<details>
<summary>Docker nativo (alternativa)</summary>

```bash
# Construir
docker build -t centag-personal:latest \
  --build-arg DIST_NAME=personal \
  --build-arg INCLUDE_FRONTEND=true \
  -f deploy/docker/Dockerfile.dist .

# Ejecutar
docker run -d --name centag \
  --env-file config/secrets/.env \
  -e CENTAG_EDITION=personal \
  -e LLM_PROXY_DB_DRIVER=sqlite \
  -e SQLITE_PATH=/app/storage/centag.db \
  -e LLM_PROXY_LOG_OUTPUT=both \
  -e LLM_PROXY_LOG_FORMAT=console \
  -p 20060:20060 \
  -v $(pwd)/deploy/docker/data/storage:/app/storage \
  -v $(pwd)/deploy/docker/data/logs:/app/logs \
  centag-personal:latest

# Detener y eliminar
docker stop centag && docker rm centag
```

</details>
</details>

---

## Capturas

<p align="center">
  <strong>Panel</strong><br/>
  <img src="docs/assets/readme/screenshot-dashboard.png" alt="Panel" width="900" />
</p>

<p align="center">
  <strong>Editor visual de pipelines</strong><br/>
  <img src="docs/assets/readme/screenshot-pipeline-visual-editor.png" alt="Editor visual de pipelines" width="900" />
</p>

<p align="center">
  <strong>Configuración de Agent</strong><br/>
  <img src="docs/assets/readme/screenshot-agent-config.png" alt="Configuración de Agent" width="900" />
</p>

<p align="center">
  <strong>Uso de tokens y facturación</strong><br/>
  <img src="docs/assets/readme/screenshot-token-usage.png" alt="Uso de tokens y facturación" width="900" />
</p>

---

## Modos de proxy — listos para usar

Plantillas de pipeline por escenario (cambiar con atajos `#`):

| Modo | Atajo | Descripción |
|------|-------|-------------|
| Scheduling inteligente | (por defecto) | Enrutado por compatibilidad de modelo y carga del backend |
| Proxy transparente | `#t` | Reenvío tal cual — alto rendimiento sin pérdida, sin inyectar system prompt |
| Backend directo | `#d` | Egress fijo + system prompt gestionado |
| Fallback | `#f` | Degradación automática entre backends |
| Router | `#r` | Enrutado multi-rama por intención (escenario / modelo) |
| Auditoría | `#a` | Generate → quality audit → feedback |
| Optimizar | `#o` | Generate → optimización de contenido |
| Agregador | `#ag` | Generación multi-modelo en paralelo → merge |
| Firewall de seguridad | `#sec` | Safety → generate → PII redact |
| Pasarela RAG | `#rag` | Generación aumentada por recuperación con caché primero |
| Geo routing | `#geo` | Enrutado regional por reglas |
| Pi Agent | `#pi` | Tareas de código → sandbox; Q&A → LLM |
| CI/CD Webhook | — | Disparar pipelines desde sistemas externos |

Lo que más brilla son los **pipelines personalizados** — diseña tu propio DAG en el lienzo.

---

## Documentación

| Tema | Enlace |
|------|--------|
| Índice completo | [docs/README.md](docs/README.md) |
| Estándar de plugins de pipeline | [docs/guide/pipeline-plugin-standard.md](docs/guide/pipeline-plugin-standard.md) |
| Guía de plugins Processor | [docs/guide/processor-plugins.md](docs/guide/processor-plugins.md) |
| Variables de pipeline | [docs/guide/pipeline-variables.md](docs/guide/pipeline-variables.md) |
| Modos de proxy | [docs/guide/proxy-modes.md](docs/guide/proxy-modes.md) |
| Configuración de backends | [docs/guide/backend-configuration.md](docs/guide/backend-configuration.md) |
| Proxy local / wrap | [docs/guide/system-proxy-egress.md](docs/guide/system-proxy-egress.md) |
| Variables de entorno | [docs/guide/environment-variables.md](docs/guide/environment-variables.md) |
| Referencia de API | [docs/api/API_REFERENCE.md](docs/api/API_REFERENCE.md) |
| Seguridad | [docs/security/](docs/security/) |

---

## Comentarios y soporte

¿Preguntas o sugerencias? Abre un [GitHub Issue](https://github.com/atoml-ai/centag/issues) o escribe a **centag@atoml.com**.

---

## Contribuir

Invitamos a desarrolladores a construir y mantener Centag juntos. Bugs, funciones, documentación o adaptar más Agents — vía [Pull Requests](https://github.com/atoml-ai/centag/pulls) o [Issues](https://github.com/atoml-ai/centag/issues).

---

## Licencia

MIT License (ediciones open source: `minimal` / `personal`)
