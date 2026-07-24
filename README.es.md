# Centag

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Русский](README.ru.md) | [Español](README.es.md)

**Acceso proxy local en un clic** para Agents de coding, **gestión unificada** de backends y API keys, y **acciones de proxy configurables** por escenario (cambio, failover, pipelines)—sin reconfigurar cada herramienta por separado.

Para desarrolladores individuales: instala Centag → conecta Agents con wrap o por configuración → administra backends y políticas en la Web.

## Instalación

Elige un método. Después de instalar, ejecuta `centag` y abre **http://localhost:20060**.

### Opción 1: script de una línea (recomendado, sin Node.js)

```bash
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
```

Instala en `~/.centag/` por defecto e intenta actualizar el PATH. Luego usa `centag` / `centag wrap`.

### Opción 2: npm (si ya usas Node.js)

```bash
# Instalación global (paquete online; descarga el binario desde GitHub Releases)
npm install -g @atomlai/centag

# O probar sin cambiar rutas globales de npm
npx --yes @atomlai/centag

# Paquete offline / red cerrada
npm install -g @atomlai/centag-offline
```

Si `npm install -g` falla por permisos, usa `npx` o el script de arriba. Detalles: [apps/centag-npm/README.md](apps/centag-npm/README.md).

### Opción 3: Docker (desde el código fuente)

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # edita secretos si hace falta
./start.sh docker up                                 # por defecto: contenedor personal
```

La UI de administración sigue en http://localhost:20060. Detener: `./start.sh docker down`.

---

## Después de instalar: conectar un Agent

Objetivo: seguir usando el Agent con normalidad, con el tráfico pasando por Centag (backends compartidos, failover, medición).

1. **Abre la Web** → añade y activa al menos un backend (API key o endpoint compatible local).
2. **Agent Setup** (menú Web) — asistente para generar/escribir configs; o
3. **Proxy de proceso (recomendado — mínimos cambios en el Agent)**:

```bash
# Con Centag ya en marcha en local
centag wrap run -- opencode
# sustituye opencode por el comando de tu Agent

# Comprobación
centag wrap doctor
```

Nota: `centag wrap` **no** arranca la pasarela; solo dirige el tráfico del proceso del Agent hacia un Centag en ejecución. Guía: [system proxy egress](docs/guide/system-proxy-egress.md).

---

## Por qué Centag

| Lo que necesitas | Lo que hace Centag |
|------------------|--------------------|
| **Cambiar de backend rápido** | Gestión unificada; activar/cambiar en la Web sin reconfigurar cada Agent |
| **Failover automático + pool de API keys** | Rotación de varias keys; cambio ante límites o fallos |
| **Pipelines por escenario** | Modos configurables (passthrough, directo, scheduling, revisión…); cambiar escenario = cambiar política |
| **Uso y facturación** | Seguimiento de tokens/coste para uso personal |

En resumen: **una entrada para backends y políticas; el Agent solo escribe código.**

## Capacidades

1. **Backends / modelos + pools de API keys**  
   Configura backends y modelos en la Web; **varias API keys en pool con rotación** cuando hay límites o fallos.

2. **Editor visual de pipelines**  
   Personaliza el comportamiento del proxy en un lienzo (forward, schedule, review…); cambia políticas por escenario sin tocar el código del Agent.

3. **`centag wrap` — acceso no invasivo**  
   Lanza Agents con wrap e importa el tráfico a Centag **sin cambiar la configuración del propio Agent**.

4. **Configuración directa del Agent**  
   Apunta el API Base / Key del Agent a Centag como una pasarela LLM normal (el asistente «Agent Setup» puede ayudar a escribir configs).

Elige el camino: wrap con menos ediciones, o archivos de config con un endpoint compatible con OpenAI.

## Capturas

| Panel | Agent Setup |
|-------|-------------|
| ![Dashboard](docs/assets/readme/dashboard.png) | ![Agent Setup](docs/assets/readme/agent-setup.png) |

## Documentación

- [Índice de docs](docs/README.md)
- [Variables de entorno](docs/guide/environment-variables.md)
- [Proxy local / wrap](docs/guide/system-proxy-egress.md)
- [Referencia de API](docs/api/API_REFERENCE.md)

## Comentarios y soporte

Preguntas o sugerencias: [GitHub Issues](https://github.com/atoml-ai/centag/issues) o **centag@atoml.com**.

## Licencia

MIT License (ediciones open source: `minimal` / `personal`)
