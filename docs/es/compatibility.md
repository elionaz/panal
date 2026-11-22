# Compatibilidad y versiones

panal sigue [Semantic Versioning](https://semver.org/lang/es/). Mientras la versión sea 0.x, una
versión menor todavía puede romper cosas, y el [CHANGELOG](../../CHANGELOG.md) lo dice cuando pasa.
Desde la 1.0, las superficies de abajo solo cambian de forma compatible dentro de una versión mayor.

## Qué es estable

| Superficie | Qué se promete |
| --- | --- |
| **CLI** | Los comandos y flags de [cli.md](cli.md) mantienen su nombre y su significado. Pueden aparecer comandos y flags nuevos. La salida pensada para personas (`ls`, `profiles`, `worktrees`…) puede cambiar: no la parsees. Un código de salida distinto de cero siempre es un fallo. |
| **Variables de entorno** | `PANAL_HOME`, `PANAL_NOTIFY`, `PANAL_SESSION` y `PANAL_SOCK`, como se documentan en [cli.md](cli.md#variables-de-entorno), y las que recibe cada sesión: `PANAL_PORT` ([worktrees.md](worktrees.md)) y `PANAL_BROWSER_URL` ([configuration.md](configuration.md#navegador-para-los-agentes)). |
| **Archivos de configuración** | `profiles.yaml`, `settings.yaml`, el `.panal.yaml` de un repositorio y los archivos de temas de instrucciones (su front matter), ver [configuration.md](configuration.md) y [worktrees.md](worktrees.md): las claves mantienen su significado y las nuevas son opcionales y tienen un valor por defecto. Cada archivo dice su versión de formato (abajo). |
| **Hooks** | El comando que panal instala en los hooks de un agente (`<panal> notify --sock … --from-hook …`, ver [attention-and-activity.md](attention-and-activity.md)) sigue funcionando, así que los hooks que instaló una versión anterior no hay que reinstalarlos. |
| **Herramientas MCP** | Las herramientas del servidor MCP de panal (`panal mcp`, ver [mcp.md](mcp.md)) mantienen su nombre y sus argumentos, y los campos de sus resultados estructurados mantienen su significado. Pueden aparecer herramientas, argumentos y campos nuevos. El texto que devuelven es para el modelo y puede cambiar. |
| **El nombre viejo** | Lo que hace que una instalación de agenterm siga funcionando (mover `~/.agenterm` y `~/.config/agenterm`, las variables `AGENTERM_*`, un comando o enlace `agenterm`, `.agenterm.yaml`) se mantiene en toda la 1.x. |
| **Protocolo del daemon** | Versionado (abajo). Es como se hablan la app, la CLI y el daemon; no es una API pública, pero versiones de panal que no se entienden lo detectan y dicen qué hacer. |

Fuera de esto: `daemon.log` (su contenido cambia libremente), los archivos de ejecución
`restart.json`, `d.pid` y `scrollback/`, las copias `.bak` y `.corrupt-*`, y todo lo que está bajo
`internal/` en el código.

## Formatos de archivo

Cada archivo que escribe panal dice qué formato usa:

| Archivo | Dónde | Cómo se escribe la versión | Actual |
| --- | --- | --- | --- |
| `settings.yaml` | `~/.config/panal/` | clave `version:` en el nivel superior | 1 |
| `profiles.yaml` | `~/.config/panal/` | `version:` junto a la lista `profiles:` | 1 |
| `projects.json` | `~/.config/panal/` | `{"version": 1, "projects": [...]}` | 1 |
| `worktrees.json` | `~/.panal/` | `{"version": 1, "worktrees": [...]}` | 1 |
| `sessions.json` | `~/.panal/` | `{"version": 1, "sessions": [...]}` | 1 |
| `messages.json` | `~/.panal/` | `{"version": 1, "messages": [...]}` (desde 0.6.0) | 1 |
| `trust.json` | `~/.config/panal/` | `{"version": 1, "trusted": [...]}` | 1 |
| `.panal.yaml` | la raíz de un repositorio | clave `version:` de primer nivel | 1 |
| `history/<id>.json` | `~/.panal/` | un campo `"version"` en cada entrada (las que se guardaron antes de versionarlas no lo tienen: formato 1) | 1 |

- **Los archivos de antes de la 0.4.0** no tienen versión: son formato 1 y se leen tal cual
  (`profiles.yaml` como una lista, los JSON como arrays). panal añade la versión la próxima vez
  que guarda el archivo. No tienes que cambiar nada a mano.
- **Un formato más nuevo del que entiende esta versión** significa que el archivo lo escribió un
  panal más nuevo (por ejemplo, después de volver a una versión anterior). panal lo rechaza
  con un error que lo dice (`… was written by a newer panal (format 2; this build reads up to 1):
  update panal`) y **nunca lo sobrescribe**, así que no se pierde nada. `settings.yaml` es la
  excepción al leer: se aplica lo que esta versión entiende, en la medida en que puede leerlo, pero tampoco se guarda encima.
- La versión de un formato solo sube cuando un panal anterior leería mal el archivo. Añadir una
  clave opcional no la cambia. Un panal anterior ignora las claves que no conoce; `settings.yaml`
  las conserva al guardarse, pero los demás archivos se reescriben con lo que esa versión entiende,
  así que las pierde en su siguiente guardado (y la versión más nueva usa entonces sus valores por
  defecto).
- Una versión que no es un número entero positivo se trata como una más nueva: el archivo se rechaza y no se
  toca.

### Volver a una versión anterior

La 0.3.x y anteriores no conocen los formatos con versión: cuando la 0.4.0 ha guardado un archivo,
un panal anterior no puede leerlo. Antes de abrir la versión anterior, devuelve cada archivo a su
forma antigua (el contenido es el mismo):

- `profiles.yaml`: deja solo las entradas que hay bajo `profiles:`, como una lista en el nivel
  superior (quita `version:` y `profiles:`, y los dos espacios de sangría).
- `projects.json`, `worktrees.json`, `sessions.json`: deja solo el array que hay dentro de
  `"projects"`, `"worktrees"` o `"sessions"`.
- `settings.yaml` no necesita nada: las versiones anteriores ignoran la clave `version`.

Las copias `.bak` no sirven para esto: después de un par de guardados también están en el formato
nuevo. Si ya abriste la 0.3.x sin convertir `sessions.json`, se apartó como
`sessions.json.corrupt-<fecha>` y sus sesiones no se restauraron; convierte ese archivo y vuelve a
llamarlo `sessions.json` (con el daemon detenido) para recuperarlas.

## Protocolo del daemon

La app y la CLI hablan con el daemon por su socket con JSON delimitado por saltos de línea (ver
[architecture.md](architecture.md)). `daemon.ping` informa la versión del daemon y la de su
protocolo (`protocol`, ahora 1; los daemons anteriores a la 0.4.0 no la envían y hablan la 1).

- Añadir métodos, campos opcionales o tipos de evento mantiene la versión del protocolo. Quitarlos,
  renombrarlos o cambiar lo que significan, o los frames binarios de `session.attach`, la sube.
- Después de una actualización, la ventana corre la versión nueva mientras el daemon, que sobrevive
  a la ventana, puede seguir siendo el anterior: la app lo dice y ofrece **Reiniciar ahora**, que
  suspende y reanuda las sesiones.
- Si el daemon es **más nuevo** que la app (un protocolo más nuevo), reiniciarlo desde la app lo
  bajaría de versión, así que la app te pide actualizar panal.
- Un comando que el daemon no conoce (un daemon anterior) falla con un mensaje claro (código
  `unknown_method`): reinicia el daemon, desde la app o con `panal stop`.
