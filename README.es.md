<p align="center">
  <img src="assets/icon/panal-icon-rounded.svg" alt="panal" width="96">
</p>

# panal

[English](README.md) · **Español**

> panal se llamaba **agenterm** hasta la 0.12.0. Las instalaciones existentes se migran solas; ver el
> [changelog](CHANGELOG.md).

Workspace de escritorio para agentes de código, organizado por proyecto. Las pestañas de una
terminal no se pueden agrupar por proyecto, y cuando lanzas varios agentes (Claude Code, codex,
opencode, Gemini, Cursor…) sobre el mismo repo es difícil saber cuál está trabajando, cuál terminó
y cuál está esperando tu aprobación. panal agrupa las sesiones por proyecto, mantiene los agentes
vivos aunque cierres la app y te avisa cuando alguno te necesita.

<p align="center">
  <img src="docs/img/screenshot-es.png" alt="panal: tres proyectos en la barra lateral, cada sesión con su modelo, rama, cambios y estado en vivo (trabajando, una aprobación esperando, terminada), un mensaje esperando, el navegador propio de una sesión abierto (globo) y una sesión que continúa a otra (⇠ Changelog draft); a la derecha, la terminal del agente elegido" width="900">
</p>

## Una tarea, de principio a fin

panal acompaña una tarea desde el primer prompt hasta el merge, en una sola ventana:

1. **Empieza en su propio worktree.** **+** en un proyecto, eliges el agente y *Worktree nuevo*:
   el agente tiene su propia rama y carpeta, así los agentes que trabajan en paralelo nunca tocan
   los archivos de otro. Un `.panal.yaml` copia el `.env` y otros archivos sin versionar y corre una
   preparación (`npm install`) antes de que arranque el agente.
2. **El agente trabaja.** La barra lateral muestra qué está haciendo ("Editando main.go") y te
   avisa, con una notificación, cuando pide una aprobación, tiene una pregunta o termina su turno.
3. **Revisas en Cambios.** El diff de todo lo que cambió la rama. Haz clic en un número de línea
   para dejar una nota en esa línea y mándale todas tus notas al agente en un solo mensaje; las
   corrige en la misma sesión.
4. **Abres el pull request.** Desde Cambios: se hace push de la rama y se abre el pull request en
   GitHub o GitLab, con título y descripción sugeridos a partir de los commits. Sus checks,
   revisión y comentarios aparecen en Cambios: *Pedirle a &lt;sesión&gt; que los arregle* le devuelve
   al agente los checks que fallan, y *Enviar todos a &lt;sesión&gt;* los comentarios de la revisión.
5. **Merge y limpieza.** Haz el merge en el host desde Cambios (o en la base local). Cuando el pull
   request está mergeado, aquí o en GitHub, panal ofrece eliminar el worktree y su rama;
   `panal worktree rm --merged` limpia lo que quede.

Todo es git normal y tu propio login de `gh` / `glab`. Detalles en
[docs/es/worktrees.md](docs/es/worktrees.md).

## Funciones

- **Funciona con cualquier CLI de agente**, no solo con unos pocos. Un catálogo integrado (Claude
  Code, codex, opencode, Gemini, Cursor, Copilot, aider, Amp, goose, Qwen, Crush, Kiro, Hermes,
  shell) detecta cuáles tienes instalados, y cualquier otro comando se vuelve un agente con unas
  líneas en `profiles.yaml`.
- **Proyectos y sesiones** en una barra lateral: cada proyecto es una carpeta y cada sesión una
  terminal (xterm.js) con un agente o una shell, con su modelo (*Opus 5.5*) y su rama. Doble clic
  en una sesión para ponerle nombre;
  arrastra proyectos y sesiones para ordenarlos. Cada proyecto tiene una insignia con sus iniciales y
  un color; haz clic en ella (o clic derecho en el proyecto) para renombrarlo, elegir el color o
  poner un emoji en su lugar. Suelta archivos (p. ej. capturas) en una terminal
  para escribir sus rutas; Claude Code adjunta las imágenes soltadas.
- **Paneles divididos**: varias sesiones lado a lado o apiladas (⌘D / ⌘⇧D), con divisores que se
  arrastran; la disposición se guarda por proyecto.
- **Comodidades de terminal**: búsqueda en el scrollback (⌘F), ⌘-clic para abrir URLs, nueve temas integrados con nombre
  de discos (*Turn On the Bright Lights*, *Room on Fire*, *Is This It*, *Funeral*, *Silent Alarm*,
  *Z*, *It Still Moves*, *Alphabetical* y *Hot Fuss*), fuente, tamaño, interlineado y cursor, y la
  app en claro u oscuro (o siguiendo a macOS).
- **Las sesiones sobreviven a la app**: un daemon es dueño de las PTYs; cierras la ventana, la
  reabres y todo sigue ahí con su scrollback. La CLI y la app comparten el mismo daemon. Si el
  propio daemon se reinicia, las sesiones vuelven suspendidas con su scrollback y se reanudan con
  un clic (Claude Code y codex continúan la misma conversación).
- **Detección de atención**: aprobaciones, preguntas, turnos terminados y errores (hooks del agente
  o señales de terminal), con notificaciones nativas de macOS, badge del Dock y marcas en la sidebar.
- **Actividad en vivo**: qué está haciendo cada agente ("Editando main.go"), cronómetro del turno,
  aviso de «¿trabado?», barras de progreso (OSC 9;4) y una **Vista general** de todas las sesiones
  (⌘0).
- **Worktrees de git por sesión**: los agentes nuevos empiezan por defecto en su propia rama y
  carpeta, así los que trabajan en paralelo nunca comparten cambios. Un `.panal.yaml` en el
  repositorio copia `.env` y otros archivos fuera de git a cada worktree nuevo y corre un setup
  (`npm install`) antes de que arranque el agente. La pestaña **Cambios** muestra el diff y permite
  hacer merge a la base o eliminar el worktree de forma segura.
- **Pull requests en GitHub y GitLab** desde la pestaña **Cambios**, para worktrees y para la rama
  en la que está tu carpeta principal: estado, checks y review, los comentarios (los de línea llevan
  al archivo), merge en el servidor y crear uno nuevo con título y descripción sugeridos. Usa tu
  propia sesión de `gh` / `glab`; panal no guarda tokens.
- **Mensajes entre sesiones**: los agentes de un proyecto pueden escribirse
  (`panal msg send`, `panal wait`); Claude Code recibe sus mensajes solo, los demás agentes
  los leen con `panal msg read`. Cada proyecto es **automático** (los mensajes llegan solos y
  despiertan a su sesión) o **manual** (esperan a que los entregues tú). Ves quién le escribe a quién
  en el momento, en la barra lateral y en la conversación del proyecto (💬), cada sesión con su
  color. Los mensajes nunca salen de su proyecto y el daemon decide quién envió cada uno.
- **Continuar con otro agente**: pasa el trabajo de una sesión a una nueva (cualquier agente, en la
  misma carpeta o en un worktree nuevo) o a una que ya está corriendo, con un resumen editable de lo
  pedido, los cambios y el final de su terminal; la sesión anterior queda plegada bajo la nueva.
- **Instrucciones para tus agentes**, por temas (seguridad, arquitectura, git…): cada una se carga
  siempre o solo cuando el trabajo la necesita, para todos los proyectos o solo algunos. Claude
  Code y codex las reciben al empezar cada sesión, sin tocar su configuración ni tu repositorio.
- **Un navegador propio por sesión**: los agentes prueban páginas y toman capturas en un Chrome
  aislado (perfil, ventana y puerto propios) que solo se abre cuando hace falta, nunca en el tuyo.
  Claude Code y codex lo reciben como herramientas de navegador.
- **Sesiones pasadas, con búsqueda**: cuando termina una sesión de agente, quedan sus prompts y el
  texto de su terminal (con los secretos tapados). Búscalas todas (⌘⇧F o el reloj de la barra lateral), lee una y **Retómala**:
  su conversación sigue en una sesión nueva, en la misma carpeta.
- **Ajustes en la app**: activar, ordenar y editar agentes, instalar los que todavía no tienes, y
  configurar el idioma, la apariencia, las notificaciones, git y el navegador de los agentes.
- **Actualizaciones integradas** (Sparkle): una comprobación diaria en segundo plano, **Buscar
  actualizaciones…** en el menú de la app y una sección *Actualizaciones* en *Ajustes → General*,
  también en las instalaciones con Homebrew.
- **En inglés y español**: la app sigue los idiomas preferidos de macOS (con inglés como respaldo)
  o el idioma que elijas en *Ajustes → General*.
- **macOS y Linux**: una app universal para macOS y una experimental para Linux (AppImage y .deb,
  amd64 y arm64); ver [Linux](#linux).

## Cómo funciona

- **Un daemon es dueño de las terminales.** Cada sesión es una PTY real que arranca el daemon de
  panal (sin tmux ni screen). La app y la CLI `panal` son clientes de ese daemon por un socket
  unix local, así que cerrar la ventana, o la app entera, no detiene a ningún agente.
- **Las sesiones sobreviven a los reinicios.** El daemon guarda el scrollback de cada sesión en
  disco. Si el propio daemon se reinicia (una actualización, un fallo), las sesiones vuelven
  suspendidas y se reanudan con un clic; Claude Code y codex continúan la misma conversación.
- **La atención la avisa el agente cuando puede.** Claude Code avisa con hooks y codex con su ajuste
  de notify; cualquier otra CLI con señales de terminal (campana, notificaciones OSC) o con patrones
  que configures. panal pasa sus hooks como flags de arranque y nunca edita la configuración de
  tu agente. Detalles en [Cómo se detecta la atención](#cómo-se-detecta-la-atención).
- **Los worktrees son git normal.** Una sesión en worktree es `git worktree add` en su propia rama
  `agent/…`; nada sale de tu máquina salvo que hagas push o abras un pull request.

Más en [docs/es/architecture.md](docs/es/architecture.md).

## Capturas

| | | |
|---|---|---|
| <img src="docs/img/overview-es.png" alt="La Vista general: una tarjeta por sesión con su estado, modelo, rama, actividad y el final de su terminal" width="280"> | <img src="docs/img/changes-es.png" alt="La pestaña Cambios de una sesión con worktree: archivos cambiados, diff lado a lado y Merge a main / Crear pull request" width="280"> | <img src="docs/img/messages-es.png" alt="La pestaña Mensajes: un mensaje enviado a otra sesión y su respuesta esperando a ser entregada" width="280"> |
| **Vista general** (⌘0): todas las sesiones de un vistazo | **Cambios**: el diff, merge o pull request | **Mensajes** entre las sesiones de un proyecto |
| <img src="docs/img/instructions-es.png" alt="Ajustes, Instrucciones: temas que se cargan siempre o cuando hace falta, para todos los proyectos o solo algunos, y qué agentes los reciben" width="280"> | <img src="docs/img/handoff-es.png" alt="Continuar con otro agente: una sesión nueva o una existente, el agente, dónde trabaja y un resumen editable del trabajo hasta ahora" width="280"> | |
| **Instrucciones** para tus agentes, por temas | **Continuar con** otro agente | |

## Instalación

Proyecto en etapa temprana (0.x): espera asperezas. La app necesita **macOS 13 (Ventura) o
posterior**; es un build universal (Apple Silicon e Intel), aunque se prueba sobre todo en Apple
Silicon. También se publica una app para **Linux** (AppImage y .deb, amd64 y arm64), experimental:
ver [Linux](#linux).

### Homebrew

```sh
brew install --cask elionaz/tap/panal
```

Instala `panal.app` en `/Applications` y pone la CLI `panal` en tu `PATH` (la CLI viene dentro
de la app, en `panal.app/Contents/Helpers/panal`). La app mantiene ambas al día
([Actualizar](#actualizar)); `brew uninstall --zap --cask panal` borra además `~/.panal` y
`~/.config/panal`.

### Descarga

Cada [release](https://github.com/elionaz/panal/releases) trae:

- `panal-<versión>-macos-universal.dmg` / `.zip`: la app (abre el dmg y arrástrala a
  Aplicaciones, o descomprime el zip);
- `panal-<versión>-darwin-universal.tar.gz`: solo la CLI para macOS;
- `panal-<versión>-linux-{amd64,arm64}.AppImage` / `.deb`: la app para Linux (experimental, ver
  [Linux](#linux));
- `panal-<versión>-linux-{amd64,arm64}.tar.gz`: solo la CLI y el daemon para Linux;
- `checksums.txt`: el SHA-256 de cada archivo (`shasum -a 256 -c checksums.txt`).

Las releases van firmadas con un Developer ID y notarizadas por Apple. Si las notas de una release
dicen que es un **build sin firmar**, macOS se negará a abrirla; después de moverla a
`/Applications`, ejecuta `xattr -dr com.apple.quarantine /Applications/panal.app`.

### Linux

Experimental: se compila y empaqueta en Ubuntu 24.04 y está poco probado. Necesita **WebKitGTK 4.1**
y GTK 3 de tu distribución (Ubuntu 22.04+, Debian 12+, Fedora 39+…), que los paquetes no incluyen.

- **.deb** (Ubuntu, Debian): `sudo apt install ./panal-<versión>-linux-amd64.deb` instala la app
  (*panal* en el menú de aplicaciones, `panal-app`) y pone la CLI `panal` en tu `PATH`.
- **AppImage** (cualquier distribución): `chmod +x panal-*.AppImage` y ejecútalo (instala antes
  `libwebkit2gtk-4.1-0` si tu sistema no lo tiene). La primera vez copia su CLI a
  `~/.local/share/panal/bin/panal`, desde donde corre el daemon (sigue vivo al cerrar la
  ventana); agrega esa carpeta a tu `PATH` para usar la CLI.

Lo que cambia respecto a macOS:

- **Los atajos usan Ctrl+Shift** donde macOS usa ⌘ (Ctrl+Shift+N, Ctrl+Shift+1…), así Ctrl+letra
  sigue llegando a la terminal; ⌘⇧D es **Ctrl+Shift+E** (dividir hacia abajo) y ⌘⇧↑/↓ es
  **Ctrl+Shift+RePág/AvPág**. Ctrl+Shift+C / Ctrl+Shift+V copian y pegan en la terminal; **Ctrl-clic**
  abre los enlaces. La app muestra las teclas de Linux en sus etiquetas.
- **Las notificaciones** van por el servicio de notificaciones del escritorio (D-Bus); al hacer clic
  se abre su sesión. No hay contador en el Dock.
- **Las actualizaciones** no las instala la app: busca versiones nuevas cada pocas horas y el aviso
  lleva a la descarga.

Los cambios de cada versión están en [CHANGELOG.md](CHANGELOG.md) (solo en inglés).

### Compilar desde el código fuente

Requisitos:

- **macOS** (la app y las notificaciones son exclusivas de macOS).
- **Go 1.27.4+** (ver `go.mod`).
- **Node 24+** y npm (para el frontend; es la versión que usa el CI).
- **CLI de Wails v2**: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1` (`wails doctor`
  debe pasar).

```sh
git clone https://github.com/elionaz/panal.git
cd panal

# App de escritorio (incluye el daemon)
cd app && wails build          # -> app/build/bin/panal.app
open build/bin/panal.app    # o cópiala a /Applications

# CLI (opcional)
cd .. && go build -o panal ./cmd/panal
```

La primera vez que abras la app, macOS pregunta si permites notificaciones. Si lo rechazaste:
*Ajustes del Sistema → Notificaciones → panal* → "Permitir notificaciones" (**Ajustes →
Notificaciones** en panal muestra el permiso, abre esa página y envía una notificación de prueba). Sin ese permiso (o
desde un daemon fuera del `.app`) las notificaciones no se muestran, salvo que actives la
alternativa con AppleScript (`notify.fallback: osascript`; entonces aparecen como *Editor de
Scripts*). La CLI incluida en la app (la que Homebrew pone en tu `PATH`) arranca el daemon como la
app, así que también tiene notificaciones nativas.

## Actualizar

- **Homebrew**: la app se actualiza sola, igual que la descargada (abajo). Un simple
  `brew upgrade` no la toca, como con cualquier cask que se actualiza solo; `brew upgrade --cask
  panal` (o `--greedy`) sí la actualiza. Las instalaciones de antes de la 0.4.2 necesitan un
  último `brew upgrade --cask panal` para llegar ahí.
- **App descargada** (dmg/zip): panal busca una release nueva una vez al día y ofrece instalarla;
  **panal → Buscar actualizaciones…** lo comprueba al momento. Si encuentra una versión nueva
  mientras trabajas, un aviso te lo indica (**Actualizar**, o **Reiniciar para actualizar** cuando
  ya se descargó; **Más tarde** lo oculta para esa versión hasta que se descargue) y también
  aparece en *Ajustes → General → Actualizaciones*. Reiniciar para actualizar solo relanza la ventana: el daemon en
  segundo plano y tus sesiones siguen corriendo. En *Ajustes → General →
  Actualizaciones* puedes desactivar la comprobación diaria o dejar que las actualizaciones se
  descarguen e instalen solas (al salir). Antes de instalarse se verifican con una firma EdDSA y la firma
  de código Developer ID de la app (las releases que se ofrecen desde la app siempre van
  notarizadas).
- Los builds desde el código fuente (`wails build`) no incluyen el actualizador.

Las sesiones corren en un daemon en segundo plano que sobrevive a la app, así que justo después de
actualizar el daemon sigue siendo la versión anterior. La app lo reinicia sola cuando ningún agente
está en medio de un turno (las sesiones en marcha se suspenden y se reanudan, diga lo que diga
`sessions.restore`; las que habías dejado suspendidas siguen así). Si hay un agente trabajando,
muestra en su lugar un aviso con **Reiniciar ahora**, para que lo hagas cuando te convenga, o más
tarde desde una terminal con `panal stop --force`.

## Desinstalar

Primero detén el daemon: cierra la app y luego `panal stop` (se niega si hay sesiones en marcha;
`panal stop --force` las termina). Después:

- **Homebrew**: `brew uninstall --cask panal` quita la app y la CLI y conserva tus datos.
  `brew uninstall --zap --cask panal` también los borra: `~/.panal`, `~/.config/panal` y
  los archivos de panal en `~/Library` (cachés, preferencias, datos de WebKit, estado guardado de
  la ventana).
- **A mano**: borra `/Applications/panal.app`, el enlace de la CLI si lo creaste
  (`/usr/local/bin/panal`) y, para borrar también tus datos, `~/.panal` (estado del daemon,
  scrollback y los worktrees de panal), `~/.config/panal` (ajustes, perfiles, proyectos) y
  `~/Library/{Caches,WebKit}/com.elionaz.agenterm`,
  `~/Library/Preferences/com.elionaz.agenterm.plist` y
  `~/Library/Saved Application State/com.elionaz.agenterm.savedState`.

**Antes de borrar `~/.panal`**, revisa `~/.panal/worktrees` (o tu `git.worktree_dir`): ahí están los
worktrees que creó panal, y el trabajo sin commitear que tengan se pierde con él (`--zap` incluido). Sus ramas
quedan en tus repositorios; `git worktree prune` limpia después las referencias.

No hay nada más que deshacer: panal no cambia la configuración de tus agentes (los hooks se
añaden a cada sesión que abre, no se instalan) ni toca las carpetas de tus proyectos.

## Inicio rápido

**App.** La primera vez, una pantalla de bienvenida muestra qué CLIs de agentes se encontraron, te
deja permitir las notificaciones y explica cómo poner el comando `panal` en tu `PATH`. Haz clic en
**+ Añadir tu primer proyecto** (o en **+ Proyecto** de la barra lateral) y elige una carpeta. Con el **+** junto al proyecto eliges un agente
y si corre en un worktree nuevo (por defecto) o en la carpeta del proyecto; **⌘N** lanza el primer
agente de la lista.
Cuando un agente te necesita verás un punto ámbar en la sidebar y una notificación.

**Varios agentes en un proyecto.** Cada agente nuevo tiene por defecto su propio **worktree** (su
rama y su carpeta), así su trabajo queda aparte; una **Shell** simple empieza en la carpeta del
proyecto. Las sesiones que sí comparten la carpeta del proyecto (**Misma carpeta** en el menú **+**)
comparten su rama y sus cambios: lo que edita o commitea un agente lo ven los demás. El trabajo
vuelve con **Merge a la base** o **Crear pull request** en la pestaña Cambios. Más en
[docs/es/worktrees.md](docs/es/worktrees.md#varios-agentes-en-un-proyecto).

**CLI.** Cualquier comando arranca el daemon si no está corriendo:

```sh
panal project add ~/Dev/miapp           # -> <projectId>  miapp  /Users/yo/Dev/miapp
panal profiles                          # agentes del catálogo, instalados o no
panal spawn miapp claude                # -> <sessionId>
panal spawn --worktree miapp codex      # -> <sessionId>  <rama>  <carpeta>
panal ls                                # estado, atención, actividad, rama…
panal attach <sessionId>                # Ctrl-\ para desacoplar; la sesión sigue viva
panal stop                              # se niega si quedan sesiones vivas (usa --force)
```

Referencia completa: [docs/es/cli.md](docs/es/cli.md).

## Cómo se detecta la atención

Todo se configura **solo para las sesiones que lanza panal**, con flags en la línea de comandos y
archivos generados en `~/.panal/hooks/`. Nunca se modifican tus configuraciones
(`~/.claude/settings.json`, `~/.codex/config.toml`…) ni la carpeta del proyecto.

| agente | mecanismo | qué detecta |
|---|---|---|
| Claude Code | hooks vía `claude --settings <archivo generado>` (se suman a los tuyos) | trabajando + actividad, aprobación, pregunta, terminó, error |
| codex | `codex -c notify=[…]` + notificaciones OSC 9 del TUI | terminó (`notify`); aprobaciones y preguntas (OSC 9) |
| los demás | señales de terminal: BEL, OSC 9, OSC 777, OSC 99 y `attention_patterns` | esperando tu respuesta |

Detalles (estados, *ack*, «¿trabado?», progreso, notificaciones con la app cerrada):
[docs/es/attention-and-activity.md](docs/es/attention-and-activity.md).

## Configuración

| | por defecto | con `PANAL_HOME=/x` |
|---|---|---|
| socket, pid, log del daemon | `~/.panal/{d.sock,d.pid,daemon.log}` | `/x/…` |
| `profiles.yaml`, `settings.yaml`, `projects.json` | `~/.config/panal/` | `/x/` |
| hooks generados, worktrees, `worktrees.json` | `~/.panal/{hooks,worktrees,worktrees.json}` | `/x/…` |

`profiles.yaml` y `settings.yaml` son opcionales y la app los edita desde **Ajustes** (⌘,).

**`profiles.yaml`** se superpone al catálogo por `id`: cambias solo los campos que pongas, creas
variantes con `extends`, ocultas agentes con `enabled: false` y el orden de la lista es el del menú
(el primero es el de ⌘N):

```yaml
version: 1
profiles:
  - id: claude-yolo                 # variante: hereda todo de claude
    name: Claude (sin permisos)
    extends: claude
    args: ["--dangerously-skip-permissions"]
  - id: claude
    env: {CLAUDE_CODE_USE_BEDROCK: "1"}
  - id: gemini
    enabled: false
  - id: mio                         # perfil propio
    cmd: $HOME/bin/mi-agente
    attention_patterns: ['\(y/n\)']
```

**`settings.yaml`** (valores por defecto):

```yaml
version: 1
ui:
  language: auto         # auto (idiomas preferidos de macOS; si no, inglés) | en | es
  appearance: system     # colores de la app: system (sigue a macOS) | dark | light
terminal:
  font_family: '"SF Mono", Menlo, Monaco, monospace'
  font_size: 13          # 8-32 (también ⌘= / ⌘- / ⌘⌥0)
  line_height: 1.15      # 1.0-2.0
  cursor_style: block    # block | bar | underline
  theme: bright-lights   # bright-lights | room-on-fire | is-this-it | funeral | silent-alarm | z |
                         # it-still-moves | alphabetical | hot-fuss
notify:
  enabled: true          # notificaciones (app y daemon)
  sound: true            # sonido en aprobación/pregunta
  notify_on_approval: true  # por tipo: aprobaciones, preguntas,
  notify_on_question: true  # turnos terminados, errores, trabados
  notify_on_done: true
  notify_on_error: true
  notify_on_stall: true     # aviso discreto si un turno parece trabado
  fallback: none         # none | osascript: si no se pueden usar las nativas
                         # (osascript aparece como Editor de Scripts)
activity:
  stall_seconds: 180     # segundos sin salida ni eventos para «¿trabado?»; 0 lo desactiva
git:
  editor: ""             # para «Abrir en editor»; vacío: code si existe, si no Finder
  worktree_dir: ""       # dónde crear worktrees; vacío: ~/.panal/worktrees
  default_mode: worktree # worktree | same
```

Todas las reglas y el catálogo completo: [docs/es/configuration.md](docs/es/configuration.md). Worktrees
y diff: [docs/es/worktrees.md](docs/es/worktrees.md).

## Atajos de teclado

| atajo | acción |
|---|---|
| ⌘N | nueva sesión en el proyecto actual con el primer agente activado e instalado |
| ⌘1 … ⌘9 | seleccionar la sesión n del proyecto actual |
| ⌘⇧↑ / ⌘⇧↓ | subir / bajar la sesión seleccionada en su proyecto (o arrastrarla en la barra lateral) |
| ⌘0 | abrir/cerrar la vista general |
| ⌘, | abrir Ajustes |
| ⌘⇧F | buscar en las sesiones pasadas (Linux: Ctrl+Shift+H) |
| ⌘D / ⌘⇧D | dividir el panel enfocado a la derecha / abajo con una sesión nueva del mismo agente y modo |
| ⌘W | cerrar el panel enfocado (la sesión sigue en marcha; se termina con × en la barra lateral) |
| ⌘[ / ⌘] | enfocar el panel anterior / siguiente |
| ⌘⌥← ↑ → ↓ | enfocar el panel en esa dirección |
| ⌘F | buscar en la terminal enfocada; Enter / ⇧Enter o ⌘G / ⌘⇧G para siguiente / anterior |
| ⌘= / ⌘- / ⌘⌥0 | fuente de la terminal más grande / más pequeña / por defecto (se guarda en `terminal.font_size`) |
| ⌘C / ⌘V | copiar la selección / pegar en la terminal |
| ⌘-clic | abrir en el navegador la URL bajo el puntero (URLs en texto y enlaces OSC 8; solo http y https) |
| ⇧Enter | salto de línea en el prompt en vez de enviarlo (Claude Code, Codex…); envía un salto de línea (Ctrl-J) |
| Esc | cerrar la búsqueda, la vista general, Ajustes, las sesiones pasadas o la vista de un worktree huérfano |
| `Ctrl-\` | desacoplarse de `panal attach` (CLI) |

## Documentación

- [docs/es/cli.md](docs/es/cli.md): todos los comandos y flags de la CLI.
- [docs/es/configuration.md](docs/es/configuration.md): rutas, `profiles.yaml`, catálogo de agentes y
  `settings.yaml`.
- [docs/es/attention-and-activity.md](docs/es/attention-and-activity.md): hooks, señales de terminal,
  notificaciones, actividad, «¿trabado?» y progreso.
- [docs/es/worktrees.md](docs/es/worktrees.md): worktrees por sesión, diff, merge y borrado.
- [docs/es/messages.md](docs/es/messages.md): mensajes entre las sesiones de un proyecto.
- [docs/es/mcp.md](docs/es/mcp.md): las herramientas MCP de panal para los agentes.
- [docs/es/architecture.md](docs/es/architecture.md): daemon, app y CLI, protocolo, eventos y
  persistencia.
- [docs/es/development.md](docs/es/development.md): desarrollo, tests, publicación de releases,
  icono y limitaciones conocidas.
- [docs/es/compatibility.md](docs/es/compatibility.md): qué se mantiene estable entre versiones,
  versiones de formato y de protocolo, y cómo volver a una versión anterior.
- [CHANGELOG.md](CHANGELOG.md): los cambios de cada versión (en inglés).

Si cambias comandos, flags, rutas o `settings.yaml`, actualiza el README y el documento
correspondiente en `docs/`, en inglés (`README.md`, `docs/`) y en español (`README.es.md`,
`docs/es/`).

## Contribuir

Las contribuciones son bienvenidas vía pull request: lee [CONTRIBUTING.es.md](CONTRIBUTING.es.md). Para
reportar vulnerabilidades, sigue [SECURITY.es.md](SECURITY.es.md).

Licencia: [MIT](LICENSE).
