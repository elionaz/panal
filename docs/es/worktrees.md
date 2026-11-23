# Worktrees y cambios

[English](../worktrees.md) · **Español**

Cada sesión puede trabajar en la carpeta del proyecto o en su propio **worktree de git** (rama y
carpeta aparte), para que varios agentes trabajen en paralelo sobre el mismo repo sin pisarse.

Todo va por la CLI de `git` (con timeouts; nunca libgit2). Las consultas de solo lectura usan
`GIT_OPTIONAL_LOCKS=0`, así que nunca toman `index.lock` mientras el agente usa git en la misma
carpeta.

## Varios agentes en un proyecto

Las sesiones en **la misma carpeta** comparten todo lo que git guarda por carpeta: la rama actual,
los cambios sin commitear y el índice. Lo que edita o commitea un agente lo ven los demás; si uno
cambia de rama, hace stash o commitea, se lleva el trabajo a medias de los otros. El **±N** de la
barra lateral y la pestaña Cambios muestran esa carpeta, así que todas las sesiones de ahí muestran
los mismos archivos.

Para agentes trabajando en paralelo, dale a cada uno su propio **worktree**. Es lo que panal
hace por defecto (`git.default_mode: worktree`, desde 0.7.0): un agente nuevo empieza en su propia
rama y carpeta, y un **Shell** simple empieza en la carpeta del proyecto. **Misma carpeta** sigue a
un clic en el menú **+**.

- En un repositorio git, si ya hay un agente (no terminado ni suspendido) trabajando en la carpeta
  del proyecto, el menú **+** elige **Worktree nuevo** para el siguiente agente y dice qué sesión está
  ahí; igual puedes elegir **Misma carpeta**. **⌘N** y dividir un panel (⌘D) hacen lo mismo. La
  sugerencia no se recuerda como el modo del proyecto. Un **Shell** simple ni cuenta ni recibe la
  sugerencia: una terminal junto a un agente en su carpeta está bien.
- Una sesión que sí comparte la carpeta lo dice: la pestaña Cambios muestra *Carpeta compartida* con
  los otros agentes, y también el tooltip del **±N**. **Pasar a un worktree**, ahí mismo, la
  continúa en un worktree propio (ver abajo).
- `panal spawn` sin `--worktree` avisa en stderr cuando la sesión nueva comparte la carpeta
  (stdout sigue siendo solo el id).

El trabajo de cada worktree vuelve con **Merge a la base** o **Crear pull request** (abajo).

### Continuar con otro agente

Un proceso que ya corre no puede cambiar de carpeta, así que una sesión nunca se "mueve": su trabajo
lo **continúa** una sesión nueva. **⇢** en la fila de una sesión (o **Pasar a un worktree** en la
pestaña Cambios de una carpeta compartida) abre *Continuar con otro agente*:

- **Continuar en:** una sesión nueva, o **una sesión existente** del proyecto que se hace cargo del
  trabajo (viene elegida cuando otro agente trabaja en la misma carpeta): recibe el resumen como
  mensaje tuyo (ver [messages.md](messages.md)) y no se abre nada nuevo.
- **Agente** (sesión nueva): cualquiera de los instalados, incluido el mismo.
- **Dónde** (sesión nueva): la misma carpeta o worktree, o **un worktree nuevo** en una rama nueva desde el commit
  actual de la sesión (sus commits van incluidos; los cambios sin commit se quedan donde están). En
  una carpeta compartida el worktree nuevo viene elegido.
- **Resumen:** armado a partir de la sesión (los mensajes que escribiste, en Claude Code; su carpeta
  y rama, los archivos sin commit y los commits; el final de su terminal), sin secretos. Edítalo
  antes de continuar.

La sesión nueva recibe el resumen como primer mensaje (Claude Code, codex) o, en otros agentes, una
línea escrita en su prompt que apunta al archivo (`~/.panal/handoffs/<sessionId>.md`). Conserva
el nombre de la sesión anterior. La sesión anterior se cierra, para que dos agentes no editen los
mismos archivos, salvo que marques *Dejar esta sesión abierta*. Una vez cerrada, queda plegada bajo
la sesión que la continúa en la barra lateral (**⇠ nombre** en esa fila la muestra), y una sesión
que detuvo panal dice *detenida*, no un código de salida en rojo. Desde la CLI:
`panal handoff [--worktree] [--keep] [--note ARCHIVO|-] <sessionId> <perfil>`,
`panal handoff --into <sessionId> <sessionId>` (una sesión existente), `--print` muestra el
resumen.

## Modos de una sesión

- **Misma carpeta** (`same`, por defecto): el agente trabaja en la carpeta del proyecto.
- **Worktree nuevo** (`worktree`): `git worktree add -b <rama> <carpeta> <base>`.
  - **Rama**: por defecto `agent/<perfil>-<4 primeros caracteres del id de la sesión>`. El nombre se
    sanea (espacios y caracteres no válidos → `-`, sin `..`, sin `.lock`…) y, si ya existe, se le
    añade `-2`, `-3`…
  - **Base**: por defecto la rama actual del proyecto (o `main`/`master` si `HEAD` está separado).
    Puede ser cualquier rama o commit.
  - **Carpeta**: `<worktrees>/<proyecto>-<projectId>/<sessionId>/`, donde `<worktrees>` es
    `~/.panal/worktrees` (o `git.worktree_dir`) y `<proyecto>` es el nombre del proyecto en
    minúsculas y saneado. Los worktrees **nunca** se crean dentro del proyecto.
  - Se permite aunque la carpeta principal tenga cambios: el worktree parte del commit de la base.
  - Si el proyecto es una subcarpeta del repo, la sesión arranca en esa misma subcarpeta del
    worktree.
  - Errores claros si el proyecto no es un repo git (en la app la opción sale desactivada, con el
    motivo), si el repo no tiene commits todavía o si la base no existe.
- **Reabrir un worktree** (`--in-worktree <id>` en la CLI, *Abrir shell aquí* en la app): una
  sesión nueva en un worktree de panal que ya existe.

El id de un worktree es el de la sesión que lo creó.

Las integraciones de hooks funcionan igual en un worktree: Claude Code recibe `--settings` con una
ruta absoluta y no se escribe nada en el worktree ni en el proyecto. Claude Code puede pedir
confianza la primera vez para la carpeta nueva.

## Preparar los worktrees nuevos

Un worktree nuevo solo tiene lo que git sigue, así que la primera corrida de un agente suele fallar:
no hay `.env` ni `node_modules`. Un **`.panal.yaml`** en la raíz del repositorio (haz commit para
compartirlo) dice qué recibe cada worktree nuevo:

```yaml
worktree:
  copy: [.env, .env.local, config/local.json]   # desde la carpeta principal; se aceptan globs simples
  setup: npm install                             # corre en el worktree nuevo antes del agente
```

- **`copy`**: archivos o carpetas, relativos a la raíz del repositorio, que se copian desde la
  carpeta principal justo después de `git worktree add`, con su misma ruta. No se sobrescribe nada
  (un archivo que git ya puso ahí se queda), lo que no existe se salta, y nada puede apuntar fuera
  del repositorio (se rechazan rutas absolutas, `..`, `.git` y enlaces que salgan de él). En macOS
  los archivos se clonan (APFS), así que hasta una carpeta grande casi no cuesta.
- **`setup`**: un comando de shell que corre en la terminal de la sesión, en tu shell de login,
  antes de que arranque el agente; ves su salida ahí. Si falla, la terminal lo dice y espera:
  **Enter** arranca el agente igual, **Ctrl-C** termina la sesión.
- **Confianza**: un repositorio puede traer cualquier comando, así que panal solo corre un
  `setup` cuando confiaste en él. La primera vez (y cada vez que el comando cambia) la app lo
  muestra y pregunta: **Confiar y crear**, **Crear sin setup** o cancelar. Desde la CLI:
  `panal spawn --worktree --trust-setup` o `--no-setup` (sin ninguno, el spawn falla con
  `setup_untrusted` y el comando). Lo que confiaste se guarda por repositorio en
  `~/.panal/trust.json` (solo un hash del comando).
- **Solo worktrees nuevos**: retomar una sesión o reabrir un worktree no vuelve a correr nada.
- **`node_modules`**: deja que `setup` lo instale (`npm install`, `pnpm i`…) en vez de copiarlo:
  las ramas pueden tener dependencias distintas, y una carpeta compartida las mezclaría.
- **Sin el archivo**: si un repositorio no tiene `.panal.yaml` y git ignora archivos `.env*` en
  su raíz, la app ofrece copiarlos al worktree nuevo, una vez o escribiendo un primer
  `.panal.yaml` con ellos (`panal spawn` imprime una nota en su lugar).

## Un puerto por sesión

Dos agentes que arrancan un dev server quieren el puerto 3000. Cada sesión recibe
**`PANAL_PORT`**, un puerto propio: el más bajo desde 3000 que no tenga otra sesión del proyecto
y en el que nada escuche. La sesión lo conserva al retomarla. Cuando el `.panal.yaml` del
repositorio pide puertos, también se pone **`PORT`**, y la barra lateral y la barra de título lo
muestran (`:3001`; clic para copiar `localhost:3001`):

```yaml
ports:
  base: 3000            # primer puerto que se prueba
  env: [PORT]           # variables con el puerto (por defecto PORT)
```

Next.js, Express, Rails, Flask y la mayoría de los servidores leen `PORT`. **Vite no**: arráncalo
con `vite --port $PANAL_PORT` (en tu script `dev`), o lee `process.env.VITE_PORT` en
`vite.config` y agrégalo: `env: [PORT, VITE_PORT]`. Sin `ports.base` solo se pone
`PANAL_PORT`, así que nada cambia para los servidores que esperan su puerto de siempre.

## Ciclo de vida

- **Matar una sesión nunca borra su worktree.** El registro (`worktrees.json`) sobrevive a los
  reinicios del daemon.
- Estados de un worktree (`panal worktrees <proyecto>`):
  - `live`: una sesión viva lo usa;
  - `exited`: solo lo usan sesiones ya terminadas que siguen en la lista, o sesiones `suspended`
    tras un reinicio del daemon (reanudar una la vuelve a abrir en este worktree);
  - `orphan` (huérfano): ninguna sesión del daemon lo usa (las quitaste o descartaste, o ya habían
    terminado antes de que el daemon se reiniciara).
- Al listar los worktrees de un proyecto, el daemon olvida los que ya no existen en disco (y hace
  `git worktree prune`) y **adopta** los worktrees que git conoce dentro de la carpeta de worktrees
  de ese proyecto pero que el registro no tiene (por ejemplo, si se perdió `worktrees.json`).

## Cambios (diff)

- **Worktree**: `git diff <merge-base con la base>` (lo commiteado y lo no commiteado, en una sola
  lista) más los archivos sin seguimiento no ignorados, y los commits por delante
  (`git log <base>..HEAD`, máximo 200). Si la rama base desaparece se usa el commit base guardado al
  crear el worktree.
- **La base se mantiene al día**: si la rama base local va atrás de su upstream (`origin/main`: pull
  requests mergeados en el servidor y nunca traídos) y la rama ya tiene esos commits, se compara con
  el upstream, para que no cuenten como cambios de la rama. El daemon trae esa rama del upstream en
  segundo plano (`git fetch <remoto> <base>`, como mucho cada 5 minutos por repositorio y sin pedir
  credenciales nunca: sin red o sin credencial, simplemente no lo hace). Una rama cuyo pull request
  se mergeó con **squash o rebase** (sus archivos ya están iguales en la base) no muestra commits
  ni cambios, solo lo que quede sin commitear.
- **Misma carpeta**: `git diff HEAD` más los archivos sin seguimiento (contra el árbol vacío si el
  repo no tiene commits).
- Por archivo: ruta, ruta anterior (renombrados), estado `A`/`M`/`D`/`R`/`C`/`T`/`U`, líneas `+`/`−`,
  si es binario (sin contenido) y su parche.
- Límites: **2 MB** de parche en total (el resto de archivos se listan sin contenido y el diff se
  marca `truncated`); los archivos sin seguimiento de más de 1 MB, o más allá de los primeros 300,
  tampoco llevan contenido.
- **Resumen** de cada sesión (`git`: `files`, `uncommitted`, `ahead`): se recalcula de forma
  perezosa (al pasar a inactivo, esperando o terminado, y al pedir el diff), como mucho una vez cada
  3 s por sesión y nunca por cada trozo de salida. Los cambios se publican con el evento
  `session.git`.

## Acciones

- **Merge a la base** (`panal worktree merge <id>`): `git merge --no-ff` de la rama en la
  **carpeta principal** del repo. Solo si:
  - la carpeta principal está en la rama base;
  - no tiene cambios en archivos con seguimiento (los sin seguimiento no bloquean; no hay stash
    automático);
  - la base no va atrás de su upstream (actualízala antes: un merge sobre una base vieja crea un
    commit de merge que el remoto nunca recibe);
  - la rama tiene commits que la base no tiene.

  Si hay conflictos se hace `git merge --abort` y se informa qué archivos chocaron. Los cambios sin
  commitear del worktree no entran en el merge (se avisa cuántos quedaron fuera). En la app, tras un
  merge correcto se ofrece eliminar el worktree y su rama (la confirmación de siempre, con *Borrar
  también la rama* ya marcada).
- **Crear pull request** (`panal worktree pr [--no-open] <id>`): `git push -u <remoto> <rama>` y
  luego abre la página del servidor para crear el pull request hacia la base. No usa `gh`, nunca
  hace push forzado ni sube la base.
  - Remoto: el mismo al que haría push git: `branch.<rama>.pushRemote`; si no, `remote.pushDefault`;
    si no, el remoto del upstream de la rama; si no, `origin`; si no, el único remoto. Se niega si no
    hay ninguno (o si hay varios y no aplica ninguna de estas reglas).
  - Se niega si la rama no tiene commits que la base no tenga.
  - El push no es interactivo (`GIT_TERMINAL_PROMPT=0`, ssh en `BatchMode`): si tuviera que pedir
    credenciales, falla con el mensaje de git en lugar de quedarse esperando. Los credential helpers
    y `ssh-agent` siguen funcionando. Si el servidor no responde o se traba, se abandona (timeouts
    de conexión y keep-alive de ssh, una velocidad mínima en http) y el push entero tiene un límite
    de dos minutos.
  - La página: GitHub (`https://github.com/<owner>/<repo>/compare/<base>...<rama>?expand=1`) y
    GitLab (gitlab.com y servidores cuyo nombre contiene `gitlab`: *new merge request*), a partir de
    URLs de remoto http(s), ssh o `git@host:ruta` (`ssh.github.com` y alias de ssh como
    `github.com-work` cuentan como GitHub). Otros servidores: se sube la rama y el pull request
    lo abres tú allí.
  - Los cambios sin commitear del worktree no se suben (se avisa cuántos quedaron fuera).
- **Eliminar worktree** (`panal worktree rm <id>`):
  - se niega si una sesión viva lo usa;
  - sin *force* también se niega si se perderían cambios sin commitear o, con *borrar también la
    rama* (`--delete-branch`), commits sin mergear en la base; el error lista qué se perdería;
  - `--dry-run` solo hace la comprobación;
  - tras borrar: `git worktree prune` y, si se pidió, `git branch -D` (seguro: los commits sin
    mergear ya se comprobaron contra la base, salvo con *force*);
  - los commits cuyos cambios ya tiene la base (un pull request mergeado en el host con squash o
    rebase) no cuentan como sin mergear: borrar una rama mergeada no pide *force*.
- **Eliminar mergeados** (`panal worktree rm --merged [--dry-run] <proyecto>`): elimina, con sus
  ramas, todos los worktrees huérfanos cuya rama ya está mergeada en su base (con merge, squash o
  rebase) y que no tienen nada sin commitear. Nunca fuerza: uno con cambios sin commitear, o con
  un commit hecho después, se queda y aparece con el motivo.
- **Abrir en editor** (`git.editor`; por defecto `code` si está en el `PATH`, si no Finder) y
  **Abrir en Finder** (`panal worktree open [--finder] <id>`).
- **Copiar rama** (solo en la app, al portapapeles).

## Notas para el agente

En **Cambios** de una sesión de agente viva, haz click en un número de línea del diff para dejarle una
nota en esa línea al agente de la sesión (shift-click en otra línea del mismo archivo para un rango;
funciona en las dos vistas, y las líneas eliminadas se nombran como líneas de la versión anterior).
Las notas se juntan arriba de la lista de archivos, marcadas en el diff; **Enviar a &lt;sesión&gt;** las
manda todas en un solo mensaje tuyo, con el lugar de cada una (`archivo:línea`) y el código citado, y
el agente lo lee como cualquier otro [mensaje](messages.md). Las notas sin enviar se quedan mientras la
app corre, por sesión; al cerrar la app se pierden. Las shells y las sesiones terminadas no reciben
notas.

## Pull requests en GitHub y GitLab

La pestaña **Cambios** muestra el pull request de la rama, en sesiones con worktree y en sesiones
de la carpeta principal (la rama en la que está; nada en la rama base). Pasa por la CLI del propio
servidor con tu sesión: [`gh`](https://cli.github.com) para GitHub y
[`glab`](https://gitlab.com/gitlab-org/cli) para GitLab (gitlab.com y servidores cuyo nombre
contiene `gitlab`). panal no guarda tokens. Sin la CLI, o sin sesión, Cambios explica cómo
activarla (`brew install gh`, `gh auth login`) y mantiene el flujo de subir y abrir la página de
arriba.

**Una sesión que trabaja en varias ramas.** Un agente puede cambiar de rama dentro de su worktree
(`git switch -c fix/…`) y abrir un pull request por cada una. Cambios sigue la rama en la que está
el worktree ahora (el encabezado dice en cuál empezó), y *Merge a la base* y *Crear pull request*
actúan sobre ella. El worktree recuerda cada rama en la que estuvo (o desde la que hizo push), y
cuando hay más de un pull request, Cambios los lista todos, *Pull requests de esta sesión*, con su
estado y sus checks: haz clic en uno para verlo abajo (comentarios, merge en el servidor). Solo se
ofrece crear un pull request para la rama en la que está el worktree.

- **Estado**: número, abierto / borrador / mergeado / cerrado, checks (en verde, fallando con sus
  nombres, en curso; en GitLab, el pipeline) y la decisión de review. El título enlaza al servidor.
- **Comentarios**: comentarios de la conversación, reviews con texto (aprobado, cambios pedidos) y
  comentarios de línea con `archivo:línea`, que seleccionan ese archivo en el diff.
- **Enviar al agente** (sesiones de agente vivas): *Pedirle a &lt;sesión&gt; que los arregle*, junto a los
  checks que fallan, manda un mensaje con sus nombres y cómo verlos (`gh pr checks`,
  `glab ci status`); en los comentarios, *Enviar a &lt;sesión&gt;* en uno o *Enviar todos a &lt;sesión&gt;* los
  manda con su autor, `archivo:línea` y enlace, pidiéndole al agente que los atienda y haga push.
- **Merge en el servidor**: con los métodos que permite el repositorio (squash, commit de merge,
  rebase) y *borrar la rama en el servidor después*. Avisa de borradores, conflictos, checks que
  fallan o siguen en curso y merges que el servidor podría rechazar; la última palabra es del
  servidor. Nunca toca carpetas ni ramas locales (`gh pr merge` corre sin `--delete-branch`, que
  cambiaría tu carpeta a la base). El merge se rechaza si la carpeta cambió de rama desde que se
  mostró el pull request, o si es la rama de un fork con el mismo nombre (los pull requests de forks
  nunca se muestran como tuyos). Si el servidor solo lo encola (una cola de merge, el auto-merge de
  GitLab cuando pase el pipeline), Cambios lo dice y la rama se conserva hasta que se mergee de
  verdad.
- **Crear pull request**: sube la rama (nunca forzado) y lo abre hacia la base (la del worktree, o
  la rama por defecto del remoto en la carpeta principal) con título y descripción editables
  sugeridos a partir de los commits, opcionalmente como borrador.
- **Añadir las sesiones de los agentes** (activado por defecto, con vista previa): añade a la
  descripción una sección *Agent sessions* con los agentes que trabajaron en esa carpeta (sin las
  shells), su modelo y, en Claude Code, las peticiones que escribiste en cada sesión (leídas de su
  transcript; sin salidas de herramientas, subagentes ni comandos con /). Las peticiones van en
  bloques de código, así que sus enlaces y @menciones no se muestran ni avisan a nadie. Lo que
  parece un secreto (claves de API, tokens, claves privadas, `password=…`) se sustituye por
  `[redacted]`: es un filtro aproximado, revisa la vista previa antes de crear. Desmárcalo para no
  añadir la sección.

Todo se recarga al abrir Cambios, con ⟳ y cuando la rama cambia (commits, pushes); no hay
sondeo periódico. Las CLIs se buscan en el `PATH` del shell de login, como los agentes. El
repositorio es aquel al que se sube la rama: si en un esquema con fork ese es tu fork, los pull
requests se buscan (y se abren) ahí. Los comentarios se leen en páginas de 100, todos.

## En la app

- Menú **+** del proyecto: tras elegir el agente, *Misma carpeta* o *Worktree nuevo* (rama editable
  ya rellena y selector de base con la rama actual por defecto). Se recuerda la última elección por
  proyecto (para agentes; las shells siempre empiezan en la carpeta), y ⌘N la usa; si no hay
  ninguna, se usa `git.default_mode` (`worktree` por defecto). Las elecciones de antes de 0.7.0 ya
  no se recuerdan.
- Fila de la sesión: chip con la rama (sin el prefijo `agent/`): azul para el worktree propio de la
  sesión, gris para la rama actual de la carpeta del proyecto (compartida con las otras sesiones de
  ahí); después `±N` archivos cambiados. Igual en las tarjetas de la vista general, con `↑N` commits
  por delante.
- Pestañas **Terminal | Cambios (N)** en las sesiones con git: lista de archivos a la izquierda
  (estado, `+`/`−`), commits por delante, diff a la derecha con diff2html (lado a lado o unificado),
  ⟳ para recargar, aviso de diff truncado y la barra de acciones (*Abrir en editor*, *Abrir en
  Finder*, *Copiar rama*, *Merge a &lt;base&gt;*, *Crear pull request* (con una confirmación que
  muestra remoto, rama y base), *Eliminar worktree* con una confirmación que
  muestra qué se perdería, la casilla *Borrar también la rama* y otra para forzar).
- Bajo cada proyecto, **Worktrees huérfanos** (si hay): *Ver cambios*, *Abrir shell aquí* (una
  sesión `shell` nueva en ese worktree) y *Eliminar*. Los ya mergeados y limpios van primero, con
  **mergeado · se puede eliminar**, y el título ofrece **Eliminar mergeados (N)**, que pregunta una vez
  y los elimina con sus ramas.
