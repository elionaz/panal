# Configuración

[English](../configuration.md) · **Español**

## Rutas

| | por defecto | con `PANAL_HOME=/x` |
|---|---|---|
| socket, pid y log del daemon | `~/.panal/{d.sock,d.pid,daemon.log}` | `/x/…` |
| perfiles, proyectos y ajustes | `~/.config/panal/{profiles.yaml,projects.json,settings.yaml}` | `/x/…` |
| instrucciones para los agentes (un archivo por tema) | `~/.config/panal/instructions/<tema>.md` | `/x/instructions/…` |
| perfil del navegador propio de cada sesión | `~/.panal/browsers/<sessionId>/` | `/x/browsers/…` |
| archivos de hooks generados | `~/.panal/hooks/` | `/x/hooks/` |
| worktrees de las sesiones | `~/.panal/worktrees/<proyecto>-<projectId>/<sessionId>/` (o `git.worktree_dir`) | `/x/worktrees/…` |
| registro de worktrees | `~/.panal/worktrees.json` | `/x/worktrees.json` |
| sesiones a restaurar y su scrollback | `~/.panal/sessions.json`, `~/.panal/scrollback/<id>.log` | `/x/…` |
| mensajes entre sesiones | `~/.panal/messages.json` | `/x/messages.json` |
| sesiones pasadas | `~/.panal/history/<id>.{json,txt}` | `/x/history/…` |

- Los directorios se crean con permisos `0700` y el socket con `0600`.
- En macOS la ruta de un socket unix tiene un límite de unos 104 bytes: si usas `PANAL_HOME`,
  que sea una ruta corta (`/tmp/agt`, no una carpeta temporal larga de `/var/folders/…`).
- `profiles.yaml` y `settings.yaml` son opcionales. Cuando la app (o una RPC) los guarda, la
  escritura es atómica y deja la versión anterior en `<archivo>.bak`.

## Agentes: catálogo y detección

panal trae un catálogo de CLIs de agentes conocidos. Un agente cuenta como **instalado** si su
binario se encuentra en el `PATH` del daemon o en el de tu shell de login (`$SHELL -l -i -c`); esa
búsqueda se guarda en caché 5 minutos. `panal profiles scan` o **Re-escanear** en *Ajustes →
Agentes* la refrescan.

**Instalar desde la app**: un agente del catálogo que no está instalado tiene un botón **Instalar**
en *Ajustes → Agentes*. Primero muestra el comando exacto (varios son `curl … | bash`) y después lo
corre en una sesión de terminal nueva del proyecto actual, en tu shell de login, así ves su salida y
puedes escribir una contraseña si la pide; el shell queda abierto después. Una instalación que sale
bien vuelve a detectar los agentes. Solo se corren los comandos del propio catálogo (el primero de
cada sugerencia; las alternativas se muestran como texto), nunca un valor de `profiles.yaml`.

| id | agente | comando | estado | hooks | notas |
|---|---|---|---|---|---|
| `claude` | Claude Code | `claude` | `hooks` | `claude` | |
| `codex` | Codex CLI | `codex` | `heuristic` | `codex` | `notify` solo avisa el fin de turno, así que "trabajando" lo decide la heurística |
| `opencode` | opencode | `opencode` | `hooks` | `opencode` | un plugin avisa cada turno (inicio, fin, fallo), aprobaciones y preguntas; requiere opencode 2 |
| `gemini` | Gemini CLI | `gemini` | `heuristic` | `gemini` | no hay hook para un turno que falla o se cancela, así que la heurística los devuelve a idle |
| `cursor-agent` | Cursor CLI | `cursor-agent` | `heuristic` | `cursor` | la CLI no corre hooks de fin de turno desde plugins; patrones de atención `Run this command\?` y `Skip & tell the agent what to do instead` (sus menús de aprobación) |
| `copilot` | GitHub Copilot CLI | `copilot` | `heuristic` | | |
| `aider` | Aider | `aider` | `heuristic` | | patrón de atención `\(Y\)es/\(N\)o` |
| `amp` | Amp | `amp` | `heuristic` | | |
| `goose` | goose | `goose session` | `heuristic` | | |
| `qwen` | Qwen Code | `qwen` | `heuristic` | | |
| `crush` | Crush | `crush` | `heuristic` | | |
| `kiro` | Kiro CLI | `kiro-cli chat` | `heuristic` | | nombre del binario sin verificar |
| `hermes` | Hermes Agent | `hermes --cli` | `heuristic` | | patrón de atención `\[o\]nce\s+\|\s+\[s\]ession`; al reanudar se añade `-c` (la última sesión de esa carpeta). Solo se lanza si `hermes` está instalado y el perfil sigue activado; `args` y `resume_args` se pueden cambiar |
| `shell` | Shell | `$SHELL` (o `/bin/sh`) | `heuristic` | | siempre disponible |

- **estado** (`status`): `hooks` = el estado lo deciden eventos explícitos (hooks / `notify`) y la
  heurística solo se usa hasta el primer evento; `heuristic` = salida → trabajando, unos segundos sin
  salida → inactivo. Ver [attention-and-activity.md](attention-and-activity.md).
- **hooks** (`setup_hooks`): integración que panal configura al lanzar el agente.
- Los agentes no instalados (o desactivados) no salen en el menú **+** ni en ⌘N; en *Ajustes* sí,
  con el comando de instalación y un enlace.

## `profiles.yaml`

Una clave `version` (el [formato del archivo](compatibility.md#formatos-de-archivo)) y una lista
`profiles:` (los archivos de antes de la 0.4.0 son solo la lista, y se siguen leyendo). Cada entrada
tiene un `id` obligatorio y único, y se **superpone** al catálogo:

| campo | |
|---|---|
| `id` | identificador (letras, dígitos, `.`, `_`, `-`) |
| `name` | nombre visible |
| `extends` | id del perfil del que hereda (solo perfiles propios) |
| `enabled` | `false` lo oculta de los menús; por defecto `true` (no se hereda) |
| `order` | posición explícita (menor primero); sin ella cuenta la posición en la lista |
| `cmd` | comando; se expanden `$VARIABLES` |
| `args` | lista de argumentos |
| `env` | mapa `CLAVE: valor` (los valores también expanden `$VARIABLES`) |
| `status` | `hooks` o `heuristic` |
| `setup_hooks` | `claude`, `codex`, `gemini`, `opencode`, `cursor` o `none` |
| `idle_seconds` | segundos sin salida para pasar a inactivo (sobrescribe el valor del daemon, 3 s) |
| `attention_patterns` | expresiones regulares (sintaxis de Go) sobre la salida reciente sin ANSI; una coincidencia marca la sesión como esperando tu respuesta |
| `resume_args` | argumentos que se añaden (después de `args`) al reanudar una sesión de este perfil tras un reinicio del daemon cuando no hay id de sesión del agente que reanudar; p. ej. `["--continue"]`. El catálogo lo define para opencode y Cursor CLI (`--continue`) y Hermes (`-c`): la última sesión del proyecto. claude, codex, gemini, opencode y Cursor CLI reanudan por id (con su integración) y solo lo usan como alternativa. No aparece en el editor de la app (guardar desde la app lo conserva) |

### Reglas de combinación

- **Entrada con el id de un agente del catálogo**: solo cambia los campos que pongas; el resto
  (incluido `setup_hooks`) se hereda. Un campo vacío u omitido hereda; `setup_hooks: none` desactiva
  la integración; `args: []` vacía los argumentos heredados. Una entrada del catálogo no puede usar
  `extends`.
- **Id nuevo sin `extends`**: perfil propio; `cmd` es obligatorio.
- **Id nuevo con `extends: <id>`**: variante. Hereda todo del padre ya resuelto (con tus cambios
  incluidos) y sus campos se aplican encima; `args`, `env` y `attention_patterns` **reemplazan** a
  los del padre (no se mezclan). El nombre por defecto de una variante es su id.
- **Orden**: primero las entradas del archivo, por `order` o por posición; después los agentes del
  catálogo que el archivo no menciona, en el orden del catálogo. El primero activado e instalado es
  el de ⌘N. Una entrada que solo tiene `- id: x` sirve para fijar la posición de un agente del
  catálogo.
- **Errores**: un error en una entrada (regex inválida, `extends` desconocido o cíclico, falta
  `cmd`, `setup_hooks` desconocido…) solo inutiliza esa entrada; se ve en `panal profiles` y en
  *Ajustes*. Los errores de estructura (YAML inválido, id vacío o duplicado) invalidan el archivo entero: los
  comandos que lo leen fallan hasta que lo corrijas.

```yaml
version: 1
profiles:
  - id: claude-yolo           # variante
    name: Claude (sin permisos)
    extends: claude
    args: ["--dangerously-skip-permissions"]
  - id: claude
    env: {CLAUDE_CODE_USE_BEDROCK: "1"}
  - id: gemini
    enabled: false
  - id: mio                   # perfil propio
    cmd: $HOME/bin/mi-agente
    args: []
    env: {}
    status: heuristic         # hooks | heuristic
    setup_hooks: none         # claude | codex | gemini | opencode | cursor | none
    idle_seconds: 3
    attention_patterns:
      - '\(y/n\)'
```

### Edición desde la app

*Ajustes → Agentes* (⌘,, el ⚙ de la sidebar o *panal → Preferencias…*) permite activar y
desactivar, reordenar (arrastrar o ▲▼), *Editar* (nombre, comando, argumentos, entorno
`CLAVE=valor`, modo de estado, integración de hooks, segundos de inactividad y patrones de
atención), *Duplicar como variante*, *+ Agente personalizado*, *Borrar* (perfiles propios) o
*Restablecer* (agentes del catálogo modificados) y *Re-escanear*.

Al guardar desde la app (RPC `profile.save`, `profile.delete`, `profile.reorder`) el archivo se
reescribe en **formato canónico**: una entrada por perfil en el orden del menú, solo con los campos
que difieren de lo heredado (los agentes del catálogo sin cambios quedan como `- id: x`) y sin
`order`. **Los comentarios no se conservan.** El archivo se relee en cada `spawn` y cada listado,
así que también puedes editarlo a mano.

## `settings.yaml`

Todas las claves son opcionales; las que faltan toman el valor por defecto.

| clave | por defecto | |
|---|---|---|
| `ui.language` | `auto` | idioma de la app: `auto`, `en` o `es` (ver [Idioma](#idioma)) |
| `ui.appearance` | `system` | colores de la app (barra lateral, barras de título, Ajustes): `system` sigue el modo claro/oscuro de macOS, o `dark` / `light`. Las terminales usan su propio `terminal.theme` |
| `terminal.font_family` | `"SF Mono", Menlo, Monaco, monospace` | una lista CSS de fuentes; se usa la primera instalada. Vacío: el valor por defecto |
| `terminal.font_size` | `13` | puntos, entre 8 y 32. ⌘= / ⌘- / ⌘⌥0 lo cambian (y lo guardan) desde cualquier parte de la app |
| `terminal.line_height` | `1.15` | múltiplo del tamaño de fuente, entre 1.0 y 2.0 |
| `terminal.cursor_style` | `block` | `block`, `bar` o `underline` |
| `terminal.theme` | `bright-lights` | paleta integrada, cada una con nombre de un disco. Oscuras: `bright-lights` (*Turn On the Bright Lights*), `room-on-fire` (*Room on Fire*), `is-this-it` (*Is This It*), `funeral` (*Funeral*), `silent-alarm` (*Silent Alarm*), `z` (*Z*). Claras: `it-still-moves` (*It Still Moves*), `alphabetical` (*Alphabetical*), `hot-fuss` (*Hot Fuss*). Los nombres retirados siguen funcionando y eligen la paleta más parecida: `panal-dark` → `bright-lights`, `panal-light` → `alphabetical`, `solarized-dark` → `z`, `solarized-light` → `hot-fuss`, `dracula` → `funeral`, `gruvbox-dark` → `is-this-it`, `nord` → `silent-alarm`. Un nombre desconocido vuelve a `bright-lights` |
| `notify.enabled` | `true` | notificaciones (de la app y del daemon) |
| `notify.sound` | `true` | sonido, solo en aprobaciones y preguntas |
| `notify.notify_on_approval` | `true` | avisar cuando un agente pide una aprobación |
| `notify.notify_on_question` | `true` | avisar cuando un agente te pregunta algo |
| `notify.notify_on_done` | `true` | avisar cuando un agente termina su turno |
| `notify.notify_on_error` | `true` | avisar cuando un turno falla |
| `notify.notify_on_stall` | `true` | aviso discreto cuando un turno parece trabado |
| `notify.fallback` | `none` | cuando no se pueden usar las notificaciones nativas (sin bundle `.app`, sin permiso): `none` no las muestra (`daemon.log` dice por qué, una vez) y `osascript` las envía con AppleScript, que macOS muestra como *Editor de Scripts* (al pulsarlas se abre Editor de Scripts, no panal). `PANAL_NOTIFY=off` en el entorno las desactiva todas igualmente. Ver [attention-and-activity.md](attention-and-activity.md#con-la-app-cerrada) |
| `activity.stall_seconds` | `180` | segundos sin salida ni eventos para marcar «¿trabado?»; `0` lo desactiva |
| `git.editor` | `""` | para «Abrir en editor»: un comando en el `PATH` (`code`, `cursor -n`, `zed`), una ruta absoluta o el nombre de una app de macOS (`Visual Studio Code`, `Cursor.app`). Vacío: `code` si está en el `PATH`, si no Finder |
| `git.worktree_dir` | `""` | dónde crear los worktrees (ruta absoluta o `~/…`). Vacío: `~/.panal/worktrees` (o `$PANAL_HOME/worktrees`) |
| `git.default_mode` | `worktree` | `worktree` o `same`: dónde el menú **+** (y ⌘N) empieza un agente nuevo en un proyecto donde aún no elegiste. Las shells siempre empiezan en la carpeta del proyecto. Antes de 0.7.0 el default era `same`; un archivo que diga `same` lo conserva |
| `sessions.restore` | `ask` | qué hace un daemon al arrancar con las sesiones que estaban vivas cuando se detuvo el anterior: `ask` (dejarlas `suspended` hasta que las reanudes o descartes), `always` (reanudarlas enseguida) o `never` (descartarlas). Ver [architecture.md](architecture.md#persistencia-y-reanudación) |
| `messages.enabled` | `true` | mensajes entre las sesiones de un proyecto (`panal msg`): si se apaga, los envíos se rechazan y los hooks de los agentes no entregan nada. Ver [messages.md](messages.md) |
| `messages.auto_deliver` | `true` | el modo de mensajes de los proyectos que no eligieron uno (cada proyecto lo cambia con el botón ⇄ / ⏸ de la barra lateral o con `panal project set --messages`): `true` es automático (el mensaje le llega a su sesión en cuanto puede recibirlo y la despierta si está inactiva, nunca sobre texto escrito sin enviar, como mucho 5 veces por sesión hasta que vuelvas a intervenir en ella y 20 por proyecto cada 10 minutos), `false` es manual (los mensajes esperan a que los entregues tú). Ver [messages.md](messages.md#automático-o-manual-por-proyecto) |
| `browser.enabled` | `true` | cada sesión tiene su propio Chrome aislado; ver [Navegador para los agentes](#navegador-para-los-agentes) |
| `mcp.enabled` | `true` | las sesiones de agente reciben el servidor MCP de panal (sesiones, mensajes, esperar, estado, historial); ver [mcp.md](mcp.md) |
| `browser.headless` | `true` | corre esos navegadores sin ventana (los miras en la pestaña **Navegador** de la sesión) |
| `browser.path` | `""` | un ejecutable compatible con Chrome; vacío: Google Chrome, Chromium, Microsoft Edge o Brave, el que esté instalado |
| `history.enabled` | `true` | guardar las sesiones de agente que terminan, para buscarlas; ver [Sesiones pasadas](#sesiones-pasadas) |
| `history.keep` | `300` | cuántas sesiones terminadas se guardan (1–5000); las más viejas se van primero |

```yaml
version: 1
ui:
  language: auto
  appearance: system
notify:
  enabled: true
  sound: true
  notify_on_approval: true
  notify_on_question: true
  notify_on_done: true
  notify_on_error: true
  notify_on_stall: true
  fallback: none
activity:
  stall_seconds: 180
git:
  editor: ""
  worktree_dir: ""
  default_mode: worktree
sessions:
  restore: ask
messages:
  enabled: true
  auto_deliver: true
terminal:
  font_family: '"SF Mono", Menlo, Monaco, monospace'
  font_size: 13
  line_height: 1.15
  cursor_style: block
  theme: bright-lights
```

Cuándo se lee:

- `notify.*` se relee en cada notificación (en la app y en el daemon).
- `activity.stall_seconds` se lee al lanzar cada sesión: afecta a las sesiones nuevas.
- `git.worktree_dir` se lee al crear cada worktree (y al listar los de un proyecto).
- `git.editor` se lee en cada «Abrir en editor».
- `sessions.restore` se lee al arrancar el daemon.
- `messages.enabled` se lee en cada mensaje enviado y en cada hook que podría entregar uno;
  `messages.auto_deliver`, cada vez que llega un mensaje o una sesión queda inactiva.
- `ui.appearance` y `terminal.*` los lee la app al cargar (y al abrir Ajustes); los cambios hechos
  en la app se aplican al momento a todas las terminales abiertas. Una edición a mano se aplica al
  recargar la ventana.

*Ajustes → Notificaciones* edita las claves `notify.*` y `activity.stall_seconds` (RPC
`settings.save`). Al guardar, el archivo se edita directamente: **se conservan** las claves
desconocidas y los comentarios, y los campos que un cliente no envía mantienen su valor. Si el
archivo no se puede interpretar se usan los valores por defecto (y se anota en el log); al guardar
se reemplaza.

*Ajustes → Git* edita `git.editor`, `git.worktree_dir` y `git.default_mode`. *Ajustes → General*
edita `ui.language`, `sessions.restore`, `messages.enabled`, `messages.auto_deliver`, `browser.enabled`, `browser.headless`, `mcp.enabled` y `history.enabled` (se guardan al momento). *Ajustes → Apariencia* edita
`ui.appearance` y `terminal.*`, con vista previa; los cambios se aplican al momento a las terminales
abiertas y se guardan enseguida.

### Idioma

`ui.language` elige el idioma de la app:

- `auto` (por defecto): el primero entre inglés y español que aparezca en los idiomas preferidos de
  macOS, luego en `LC_ALL` / `LC_MESSAGES` / `LANG` (se ignoran `C`/`POSIX`); si no, inglés;
- `en`: inglés;
- `es`: español.

Se elige en *Ajustes → General* y localiza la interfaz de la app, su menú y las notificaciones. La
salida de la CLI y los mensajes de error del daemon y de las RPC están siempre en inglés; los
rechazos que ve el usuario llevan un código de error estable, para que la app pueda mostrarlos en su
idioma. Cualquier otro valor se trata como `auto`.

## Instrucciones para los agentes

*Ajustes → Instrucciones* guarda instrucciones que panal les da a los agentes que lanza al empezar
cada sesión, organizadas por temas: seguridad, arquitectura, manejo de datos, git o cualquier otro.
Sirven para tus propias reglas ("nunca hagas force-push", "usa pnpm", "para pruebas en navegador usa
el de panal, no mi Chrome"); las instrucciones que comparte tu equipo van en el `AGENTS.md` /
`CLAUDE.md` del repositorio.

Cada tema es un archivo Markdown en `~/.config/panal/instructions/<tema>.md`, privado (`0600`) y
fuera del repositorio, con un encabezado (front matter) que también puedes editar a mano:

```markdown
---
title: Git
description: Ramas, commits, pull requests y merges
load: on_demand      # o always
projects: all        # o [<projectId>, …]
enabled: true
order: 40
---

- Trabaja en una rama, nunca en main.
- Nunca hagas force-push.
```

Un archivo sin encabezado se carga siempre, en todos los proyectos.

- **Carga.** Un tema `always` ("siempre") entra completo en la sesión al empezar. Un tema
  `on_demand` ("cuando haga falta") solo muestra ahí su nombre y su descripción; el agente corre
  `panal instructions show <tema>` para leerlo cuando el trabajo lo toca (como una skill), así los
  temas largos no ocupan contexto en cada sesión.
- **Proyectos.** Un tema aplica a todos los proyectos o solo a los que lista (en la app: *solo
  algunos*); los proyectos que ya no existen se ignoran.
- **Límites.** 16 KiB por tema; como máximo 32 KiB de texto `always` por sesión (pasado eso, los
  temas que faltan se listan para leerlos cuando haga falta). La pestaña muestra el tamaño de lo que
  recibe una sesión y avisa pasados 8 KiB.
- **Vista previa.** La pestaña muestra el texto exacto que recibe una sesión de un proyecto;
  `panal instructions [<proyecto>]` también lo imprime y `panal instructions list` lista los
  temas.
- **Claude Code** (`setup_hooks: claude`) las recibe en su hook `SessionStart`, así que también
  después de `/clear`, al reanudar o al compactar, incluso con `messages.enabled: false`.
- **codex** (`setup_hooks: codex`) las recibe como `-c developer_instructions=…` en esa invocación.
  Si tu `~/.codex/config.toml` (o `$CODEX_HOME/config.toml`) ya define `developer_instructions`,
  panal no lo toca y las sesiones de codex no reciben estas (la pestaña lo avisa).
- **Gemini CLI** (`setup_hooks: gemini`) las recibe en su hook `SessionStart` (inicio, reanudación,
  `/clear`), en carpetas en las que Gemini confía.
- **opencode** (`setup_hooks: opencode`) las recibe como archivo de instrucciones
  (`hooks/instructions/<proyecto>.md`) listado en su configuración de esa sesión.
- **Cursor CLI** (`setup_hooks: cursor`) las recibe en su hook `sessionStart`, cuando empieza un chat
  (un chat reanudado ya las tiene).
- Los demás agentes no tienen forma de recibirlas sin editar sus archivos, cosa que panal nunca
  hace; la pestaña los lista.

Los cambios aplican a las sesiones que empiecen después de guardar.

## Sesiones pasadas

Cuando una sesión de agente termina (sale, la detienes o la descartas estando suspendida), panal la
guarda en `~/.panal/history/` (`<id>.json` y `<id>.txt`, que solo tú puedes leer):

- su nombre, agente, modelo, proyecto, carpeta, ramas y cuándo empezó y terminó;
- los prompts que escribiste, en los agentes con una transcripción legible (hoy, Claude Code);
- el texto de su terminal como texto plano (sin colores ni movimientos del cursor), los últimos
  512 KB.

Antes de escribir nada, lo que parece un secreto (claves de API, tokens, claves privadas,
`password=…`) se reemplaza por `[redacted]`. Los shells no se guardan, ni las sesiones que suspende
un daemon al detenerse (no terminaron). `history.keep` (300) es cuántas se guardan: las más viejas
se van primero. `history.enabled: false` deja de guardar las nuevas (las que ya están se quedan
hasta que las borres, o borres la carpeta).

El reloj de la barra lateral (⌘⇧F; en Linux, Ctrl+Shift+H) abre **Sesiones pasadas**: buscas
mientras escribes (cada palabra debe aparecer en la sesión), en el proyecto actual o en todos, lees
una sesión con sus prompts y su texto, la **Retomas** o la **Borras**. **Retomar** abre una sesión
nueva del mismo agente en la misma carpeta que continúa su conversación (`claude --resume <id>`,
`codex resume <id>`); aparece cuando el agente informó el id de su conversación y todavía existen
el proyecto, el perfil y la carpeta. Desde el CLI:
[`panal history`](cli.md#panal-history---project-p--n-n-palabras).

## Navegador para los agentes

Los agentes prueban aplicaciones web, abren páginas y toman capturas. Si no se hace nada, usan tu
propio Chrome: mueven y cambian el tamaño de su ventana, abren pestañas entre las tuyas y ven tus
sesiones iniciadas. panal le da a cada sesión **su propio Chrome**:

- con su propio perfil en `~/.panal/browsers/<sessionId>/` (ninguna de tus pestañas, cookies,
  extensiones ni sesiones iniciadas), un tamaño de página fijo (1280×800) y su propio puerto de
  DevTools en `127.0.0.1`, desde el 9222;
- **solo se abre cuando el agente lo necesita** y se cierra cuando termina la sesión; el perfil se
  borra cuando se quita la sesión (lo que se inicie ahí dura lo que dura la sesión);
- las sesiones de **Claude Code** y **codex** lo reciben como herramientas de navegador: el servidor
  MCP [`chrome-devtools-mcp`](https://github.com/ChromeDevTools/chrome-devtools-mcp) (con `npx`,
  fijado a una versión, sin estadísticas de uso ni consultas a CrUX), agregado solo para esa
  invocación (`--mcp-config` / `-c mcp_servers…`): su configuración no se toca. Abre el Chrome de la
  sesión con la primera herramienta que use;
- todas las sesiones reciben `PANAL_BROWSER_URL` (`http://127.0.0.1:<puerto>`), y
  `panal browser` abre ese Chrome e imprime la dirección, para cualquier agente o herramienta
  (`connectOverCDP` de Playwright, `connect({browserURL})` de Puppeteer);
- las instrucciones que reciben Claude Code y codex (ver
  [Instrucciones](#instrucciones-para-los-agentes)) les dicen que lo usen y nunca tu Chrome;
- un 🌐 en la fila de la sesión indica que su navegador está abierto; clic para copiar la dirección,
  ⌥-clic para cerrarlo;
- mientras está abierto, la pestaña **Navegador** de la sesión lo muestra en vivo: sus pestañas (se ve la
  que el agente abrió o navegó por última vez; clic en otra para mirarla), la dirección de la página y un
  botón para cerrarlo. Solo muestra: el agente maneja el navegador, tus clics no llegan a la página.

Necesita un navegador compatible con Chrome (Google Chrome, Chromium, Microsoft Edge, Brave o
`browser.path`) y, para las herramientas de navegador, Node.js (`npx`); *Ajustes → General* muestra
lo que encontró. Se apaga con `browser.enabled: false`; por omisión corre sin ventana (lo miras
en la pestaña Navegador); `browser.headless: false` le da además su propia ventana.

Un agente que usa una extensión de navegador (como Claude in Chrome) igual llega a tu Chrome a
través de ella: panal no puede impedirlo, solo le da un navegador mejor y le dice que lo use.
