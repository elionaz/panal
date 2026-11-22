# Desarrollo

[English](../development.md) · **Español**

Requisitos: los de [README.es.md](../../README.es.md#compilar-desde-el-código-fuente) (Go, Node 24+ con npm y la CLI de
Wails v2). Las reglas para abrir un pull request están en [CONTRIBUTING.es.md](../../CONTRIBUTING.es.md).

## App en modo desarrollo

```sh
cd app
wails dev     # hot reload (frontend en Vite); también sirve http://localhost:34115
wails build   # -> app/build/bin/panal.app
```

Solo el frontend:

```sh
cd app/frontend
npm ci
npm run check   # svelte-check (tipos)
npm test        # tests unitarios con Vitest (src/**/*.test.ts)
npm run build   # vite build
```

## Tests

```sh
gofmt -l .                       # no debe listar nada
go vet ./... && go test -race ./...
```

Es lo mismo que corre el CI (más `npm run check`, `npm test` y `npm run build` del frontend).

- Hay un test de integración que levanta el daemon en un `PANAL_HOME` temporal y ejercita spawn,
  attach/detach/reattach con scrollback, notify, eventos, kill y apagado, y otro que lanza una shell
  en modo worktree sobre un repo temporal, hace cambios escribiendo en ella y comprueba
  `session.diff`, que `kill` conserva el worktree, el huérfano tras reiniciar el daemon, reabrirlo,
  el merge y `worktree.remove` (rechazo y force).
- `internal/daemon/persist_test.go` cubre la persistencia: una sesión de shell escribe salida, el
  daemon se detiene (`stop --force` y cancelación), un daemon nuevo sobre el mismo `PANAL_HOME`
  la lista como `suspended` con su scrollback reproducido, la reanuda (misma carpeta, línea «sesión
  restaurada») y descarta otra; las políticas `always`/`never`; un script falso con
  `setup_hooks: claude` que comprueba que `--resume <session_id>` va tras la inyección de
  `--settings`; reanudaciones fallidas y un `sessions.json` corrupto. Nunca se lanza un agente real.
- Los tests de `internal/gitx` usan repos reales en directorios temporales con la configuración de
  git aislada (`GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL` vacío, `HOME` temporal).
- `go test` nunca envía notificaciones reales: los daemons de prueba reciben un notificador vacío o
  que solo registra (un `daemon.Config` sin él no envía nada), los tests de la app usan un
  escritorio falso, y los tests de `app`, `internal/daemon` e `internal/daemoncmd` ponen
  `PANAL_NOTIFY=off` en `TestMain`. Mantenlo así en los tests nuevos.

## Probar sin tocar tu instalación

Usa un `PANAL_HOME` propio y define `PANAL_NOTIFY=off`. Es la regla para **cualquier**
instancia de prueba o demo (daemon, app o CLI que pueda arrancar un daemon):

- `PANAL_HOME` tiene que ser una ruta **corta** (`/tmp/…`): en macOS la ruta de un socket unix
  tiene un límite de unos 104 bytes y las carpetas temporales de `/var/folders/…` se pasan.
- `PANAL_NOTIFY=off` desactiva todas las notificaciones de escritorio de esa app y ese daemon
  (nativas y la alternativa con `osascript`), diga lo que diga su `settings.yaml`, para que las
  sesiones de prueba no te llenen el escritorio de avisos (ni de Editor de Scripts). El daemon lo
  hereda de quien lo arranca.

```sh
export PANAL_HOME=/tmp/agt PANAL_NOTIFY=off
go run ./cmd/panal project add .
go run ./cmd/panal spawn panal shell
go run ./cmd/panal ls
go run ./cmd/panal stop --force      # apaga solo el daemon de /tmp/agt

cd app && PANAL_HOME=/tmp/agt PANAL_NOTIFY=off wails dev   # la app contra ese mismo daemon
```

Como `PANAL_HOME` manda sobre `PANAL_SOCK`, esto también es seguro dentro de una sesión de
panal: no toca el daemon que aloja la sesión.

## Bindings de Wails

Los bindings TypeScript de los métodos de `app/app.go` están en `app/frontend/wailsjs/` y se
versionan. `wails dev` y `wails build` los regeneran; si cambias la firma de un método exportado y
solo quieres los bindings:

```sh
cd app && wails generate module
```

## Publicar una release

La versión tiene una única fuente de verdad: el tag de git `vX.Y.Z`. Los builds de release la
inyectan con `-ldflags -X` en `internal/version` (lo que muestran `panal version` y el panel
Acerca de de la app) y la escriben en el `Info.plist` del bundle (`CFBundleShortVersionString` y
`CFBundleVersion`). El `productVersion` de `app/wails.json` solo lo usa un `wails build` normal; los
builds sin ldflags dicen `dev`.

1. Mueve las entradas de `[Unreleased]` de [CHANGELOG.md](../../CHANGELOG.md) a una sección
   `## [X.Y.Z] - fecha` (y actualiza los enlaces del final), y mergéalo a `main`.
2. Etiqueta `main` y sube el tag:

   ```sh
   scripts/tag-release.sh 0.1.0      # -n: solo revisa
   ```

   Etiqueta `origin/main` (no tu checkout) cuando su CHANGELOG tiene la sección `## [0.1.0]` y el
   CI pasó en ese commit (espera mientras corre), y sube `v0.1.0`. A mano es
   `git tag -a v0.1.0 -m "panal 0.1.0" origin/main && git push origin v0.1.0`.

3. `.github/workflows/release.yml` corre con el tag (runner de macOS) y:
   - compila todo con `scripts/release-local.sh <versión>` (el mismo script que corres en local): la
     app universal (`wails build -platform darwin/universal`), la CLI universal (cgo, `lipo`)
     incluida en `panal.app/Contents/Helpers/panal`, y la CLI de Linux (`CGO_ENABLED=0`);
   - incluye Sparkle (ver [Actualizaciones integradas](#actualizaciones-integradas-sparkle)) y lo
     firma de dentro hacia fuera antes que el resto;
   - firma la CLI y luego la app con el Developer ID, hardened runtime, sello de tiempo seguro y
     `app/build/darwin/entitlements.plist`, si existen los secrets del certificado; si no, deja una
     firma ad-hoc, avisa, y las notas de la release dicen que es un build sin firmar;
   - notariza el zip y luego el dmg con `notarytool` y les grapa el ticket (*staple*), si además
     existen los secrets de App Store Connect; si Apple rechaza el envío, el job falla e imprime el
     `notarytool log`;
   - empaqueta `panal-<v>-macos-universal.{zip,dmg}`, `panal-<v>-darwin-universal.tar.gz`,
     `panal-<v>-linux-{amd64,arm64}.tar.gz` y `checksums.txt`, y los sube al run;
   - si existe `SPARKLE_PRIVATE_ED_KEY`, firma con EdDSA el zip (ya grapado) y genera
     `appcast.xml`;
   - crea la release `vX.Y.Z` en GitHub con la sección del CHANGELOG como notas (o notas generadas
     si no hay) y todos los artefactos adjuntos, `appcast.xml` incluido si el build está notarizado.
     Con `SPARKLE_PUBLIC_ED_KEY` configurada, una release sin `appcast.xml` no se marca como
     *Latest* (la URL del feed apunta a la última release). Una pre-release (`X.Y.Z-sufijo`) se publica como pre-release de GitHub: nunca es
     *Latest*, nunca se ofrece desde la app y no actualiza el cask de Homebrew;
   - si existe `HOMEBREW_TAP_TOKEN`, rellena `packaging/homebrew/panal.rb.tmpl` con la versión y
     el sha256 del zip (`scripts/render-cask.sh`) y lo sube a `Casks/panal.rb` en
     [elionaz/homebrew-tap](https://github.com/elionaz/homebrew-tap) como "panal X.Y.Z".

### Secrets

Se configuran en *Settings → Secrets and variables → Actions* del repo, o con `gh secret set`. Todos
son opcionales: sin ellos la release se publica igual, sin firmar, y el cask no se actualiza.

| secret | para qué |
|---|---|
| `MACOS_CERT_P12_BASE64` | el certificado Developer ID Application con su clave privada (`.p12`), en base64 |
| `MACOS_CERT_PASSWORD` | la contraseña del `.p12` |
| `MACOS_SIGN_IDENTITY` | opcional: el nombre de la identidad, p. ej. `Developer ID Application: Jane Doe (TEAMID)`; por defecto, la primera identidad Developer ID Application del `.p12` |
| `APPLE_API_KEY_ID` | id de la clave de la API de App Store Connect (notarización) |
| `APPLE_API_ISSUER_ID` | issuer id de App Store Connect (notarización) |
| `APPLE_API_KEY_P8_BASE64` | el `.p8` de la clave, en base64 (notarización) |
| `HOMEBREW_TAP_TOKEN` | un token de GitHub que pueda hacer push a `elionaz/homebrew-tap` |
| `SPARKLE_PRIVATE_ED_KEY` | la clave privada EdDSA de Sparkle, tal como la exporta `generate_keys -x`: firma el zip de la actualización y activa `appcast.xml` |
| `SPARKLE_PUBLIC_ED_KEY` | su clave pública (`SUPublicEDKey`); una **variable** del repositorio (no es secreta), o un secret |

Sin `SPARKLE_PUBLIC_ED_KEY` la app se compila con el actualizador desactivado; sin
`SPARKLE_PRIVATE_ED_KEY` no hay `appcast.xml`, así que a las apps instaladas no se les ofrece la
release. En ambos casos solo se avisa.

**Certificado Developer ID Application** (requiere el rol Account Holder en el Apple Developer
Program):

1. En *Acceso a Llaveros → Asistente de Certificados → Solicitar un certificado de una autoridad de
   certificación…*, escribe tu email, elige "Guardado en el disco" y guarda el
   `.certSigningRequest`.
2. En [developer.apple.com → Certificates](https://developer.apple.com/account/resources/certificates/list),
   pulsa **+**, elige **Developer ID Application** (la Sub-CA G2), sube la solicitud y descarga el
   `.cer`. Haz doble clic para añadirlo a tu llavero de inicio de sesión.
3. En Acceso a Llaveros → *Mis certificados*, clic derecho en "Developer ID Application: … (TEAMID)"
   (debe mostrar su clave privada) → **Exportar…** → `.p12` con una contraseña robusta.
4. Guárdalo:

   ```sh
   base64 -i DeveloperID.p12 | gh secret set MACOS_CERT_P12_BASE64
   gh secret set MACOS_CERT_PASSWORD          # pega la contraseña
   security find-identity -v -p codesigning   # el nombre exacto, para MACOS_SIGN_IDENTITY
   ```

   Después borra el `.p12` exportado.

**Clave de la API de App Store Connect** (notarización):

1. En [App Store Connect](https://appstoreconnect.apple.com/access/integrations/api) → *Users and
   Access → Integrations → App Store Connect API → Team Keys*, genera una clave con el rol
   **Developer**.
2. Descarga `AuthKey_<KEYID>.p8` (Apple solo deja descargarlo una vez) y anota el **Key ID** y el
   **Issuer ID** que aparece encima de la lista.
3. Guárdalos:

   ```sh
   base64 -i AuthKey_XXXXXXXXXX.p8 | gh secret set APPLE_API_KEY_P8_BASE64
   gh secret set APPLE_API_KEY_ID --body XXXXXXXXXX
   gh secret set APPLE_API_ISSUER_ID --body 00000000-0000-0000-0000-000000000000
   ```

**`HOMEBREW_TAP_TOKEN`**: un
[token personal fine-grained](https://github.com/settings/personal-access-tokens/new) con
*Repository access → Only select repositories → elionaz/homebrew-tap* y el permiso
*Contents: Read and write* (nada más). Guárdalo con `gh secret set HOMEBREW_TAP_TOKEN` y renuévalo
antes de que caduque.

### Pruebas en seco

En local (macOS, con los requisitos del README):

```sh
scripts/release-local.sh 0.1.0        # -> dist/ (ignorado por git), firma ad-hoc
codesign --verify --deep --strict --verbose=2 dist/work/panal.app
lipo -info dist/work/panal.app/Contents/MacOS/panal
scripts/render-cask.sh 0.1.0 "$(shasum -a 256 dist/panal-0.1.0-macos-universal.zip | cut -d' ' -f1)"
```

Para probar la firma, pasa una identidad de tu llavero: `SIGN_IDENTITY="Developer ID Application: …"
scripts/release-local.sh 0.1.0`. La notarización necesita además `NOTARY_KEY_PATH` (el `.p8`),
`NOTARY_KEY_ID` y `NOTARY_ISSUER_ID`.

En GitHub: *Actions → Release → Run workflow* con una versión. Compila, firma y notariza (si existen
los secrets) y sube los artefactos al run (`appcast.xml` incluido), pero no crea la release ni toca
el tap.

Para probar el appcast en local sin tu clave real, usa un par ed25519 desechable (OpenSSL 3, sin
pasar por el llavero):

```sh
k=$(mktemp -d)
openssl genpkey -algorithm ed25519 -out "$k/key.pem"
export SPARKLE_PRIVATE_ED_KEY=$(openssl pkey -in "$k/key.pem" -outform DER | tail -c 32 | base64)
export SPARKLE_PUBLIC_ED_KEY=$(openssl pkey -in "$k/key.pem" -pubout -outform DER | tail -c 32 | base64)
scripts/release-local.sh 0.2.0-dev
xmllint --noout dist/appcast.xml
```

### Actualizaciones integradas (Sparkle)

La app incluye [Sparkle 2](https://sparkle-project.org): `scripts/fetch-sparkle.sh` descarga la
versión fijada (`SPARKLE_VERSION`), la comprueba contra el SHA-256 que GitHub publica para ella y la
guarda en `.cache/sparkle/` (ignorado por git). Después `release-local.sh`:

- compila la app con `-tags sparkle` (`app/updater_sparkle_darwin.{go,m}`; cualquier otro build,
  `wails dev` y los tests incluidos, usa `app/updater_stub.go`), enlazando `Sparkle.framework` de
  forma débil (*weak*) vía `@rpath` = `Contents/Frameworks`;
- copia el framework a `panal.app/Contents/Frameworks/` y firma, en este orden y sin `--deep`,
  como documenta Sparkle para apps con Developer ID: `XPCServices/Installer.xpc`,
  `XPCServices/Downloader.xpc` (`--preserve-metadata=entitlements`), `Autoupdate`, `Updater.app`, el
  framework; luego la CLI y la app como antes;
- pone `SUPublicEDKey` en el `Info.plist` (la plantilla trae `SUFeedURL` =
  `https://github.com/elionaz/panal/releases/latest/download/appcast.xml`, comprobaciones
  automáticas diarias y actualizaciones automáticas permitidas), o la quita si
  `SPARKLE_PUBLIC_ED_KEY` está vacía: entonces la app deja el actualizador desactivado;
- tras notarizar y grapar el zip, lo firma con `bin/sign_update --ed-key-file` (la clave pasa por un
  fichero temporal 0600, nunca por la línea de comandos ni el log), verifica la firma, la comprueba
  contra `SPARKLE_PUBLIC_ED_KEY` con OpenSSL y genera `dist/appcast.xml`
  (`scripts/render-appcast.sh`: un único item, el zip de la release, `minimumSystemVersion` 13.0 y
  la sección del CHANGELOG como notas).

La release sube `appcast.xml` como asset, así que `releases/latest/download/appcast.xml` apunta
siempre a la release más reciente. Los builds sin notarizar no lo publican: Sparkle instalaría una
app que Gatekeeper luego se niega a abrir.

La app desactiva su actualizador, y lo explica en *Ajustes → General → Actualizaciones* y en
**Buscar actualizaciones…**, cuando es un build de desarrollo y cuando no tiene `SUPublicEDKey`.
Las instalaciones con Homebrew se actualizan con Sparkle como cualquier otra, y el cask declara
`auto_updates true` (desde la 0.4.3), así que un simple `brew upgrade` se la salta; `--greedy` o
nombrar el cask sí la actualizan. Llegó una release después de activar Sparkle en Homebrew porque
brew lo lee del cask del tap, no de la app instalada: las instalaciones de la 0.4.1 y anteriores
(con Sparkle desactivado) necesitaban una release a la que pasar con un simple `brew upgrade`. El
registro de brew (`Caskroom/panal/<versión>`) se queda en la versión que instaló brew después de
que Sparkle actualice la app; es lo normal en los casks con `auto_updates`, y el enlace de la CLI
apunta a una ruta fija.

**El aviso de actualización.** Sparkle se crea con un delegado del updater y otro del *user driver*
estándar (`AgtSparkleDelegate` en `updater_sparkle_darwin.m`) que informan a Go (`agtSparkleUpdate`
→ evento de Wails `updates.available` `{version, ready}`, también en
`UpdaterState().available`): `updater:didFindValidUpdate:` guarda la versión,
`updaterDidNotFindUpdate:` y la opción *Omitir esta versión* (`updater:userDidMakeChoice:…`) la
borran, y `updater:willInstallUpdateOnQuit:immediateInstallationBlock:` la marca como lista. Este
devuelve `NO`: con `YES` el bloque de instalación inmediata se podría usar, pero Sparkle detiene su
ciclo de actualizaciones hasta que la app se cierra (no encontraría releases posteriores ni
críticas, ni haría recordatorios impacientes). Con `NO` el ciclo sigue y Sparkle instala igualmente
la actualización al salir. Implementa los
[recordatorios suaves](https://sparkle-project.org/documentation/gentle-reminders): una
actualización programada muestra la ventana de Sparkle solo cuando Sparkle la pondría en primer
plano de inmediato (recién abierta la app, o tras un rato de inactividad del Mac); si no, el aviso
hace de recordatorio. `InstallUpdateNow` (tanto **Actualizar** como **Reiniciar para actualizar**)
llama a `checkForUpdates`, que trae la ventana con la actualización encontrada; si ya está
descargada, su diálogo *Instalar y reiniciar* (el daemon no se toca). Mientras Sparkle todavía la
descarga en segundo plano, `canCheckForUpdates` es `NO` y esa llamada no haría nada, así que
`InstallUpdateNow` devuelve `false` y el botón muestra *Descargando…* hasta que esté lista.
"Más tarde" se recuerda por versión y etapa (disponible / lista) en
el `localStorage` del webview. El aviso cede el sitio a los de reconexión y de reinicio del daemon.

**Generar el par de claves** (una vez, en tu Mac; la clave privada nunca va al repositorio):

```sh
SPARKLE_DIR=$(scripts/fetch-sparkle.sh)
"$SPARKLE_DIR/bin/generate_keys" --account panal          # la crea en tu llavero de sesión e imprime la pública
"$SPARKLE_DIR/bin/generate_keys" --account panal -x sparkle-private.txt
gh secret set SPARKLE_PRIVATE_ED_KEY < sparkle-private.txt
gh variable set SPARKLE_PUBLIC_ED_KEY --body "$("$SPARKLE_DIR/bin/generate_keys" --account panal -p)"
rm sparkle-private.txt
```

Guarda una copia de la clave privada (el elemento del llavero, o el texto exportado en un gestor de
contraseñas): si se pierde, las apps instaladas ya no pueden verificar actualizaciones y los
usuarios tendrán que descargar la siguiente versión a mano.

**Rotar la clave**: genera un par nuevo (otra `--account`) y publica una release firmada con la
clave privada **antigua** que lleve la clave pública **nueva**: deja `SPARKLE_PRIVATE_ED_KEY` como
está, pon la nueva en la variable `SPARKLE_PUBLIC_ED_KEY` y `SPARKLE_KEY_ROTATION=1` (variable del
repositorio; se salta la comprobación de que ambas claves son pareja). Cuando esa release esté
publicada, sustituye `SPARKLE_PRIVATE_ED_KEY` por la privada nueva y borra `SPARKLE_KEY_ROTATION`.
Si la clave antigua se filtró, hazlo cuanto antes; si se perdió, no hay camino desde la app: avisa
de que la siguiente versión hay que instalarla a mano.

**El daemon tras una actualización.** Sparkle sustituye y relanza la app, pero el daemon (arrancado
desde el ejecutable antiguo) sigue corriendo, a propósito, con las sesiones. En cada (re)conexión
la app compara la `version` de `daemon.ping` (ausente en daemons anteriores a 0.2.0) con la suya y,
si el daemon es más antiguo, muestra un aviso con **Reiniciar ahora** (con confirmación si hay
agentes trabajando). `RestartDaemon`:

1. escribe los ids de las sesiones vivas en `restart.json` del directorio de runtime;
2. detiene el daemon con force (las sesiones vivas quedan registradas como suspendidas) y espera a
   que su **proceso** termine —su archivo pid es lo último que borra—, no solo a que el socket deje
   de responder: el daemon viejo aún está matando a sus agentes, y los daemons anteriores a 0.2.0
   borran la ruta del socket como último paso, lo que dejaría huérfano a un daemon nuevo que ya
   escucha ahí. Un pid reutilizado por otro programa cuenta como terminado; un daemon que sigue
   ahí tras 20 s recibe SIGKILL (sus sesiones ya constan como suspendidas);
3. arranca el nuevo daemon, que restaura las sesiones de `restart.json` como suspendidas diga lo
   que diga `sessions.restore` (y no las reanuda solo con `always`), y borra el archivo;
4. reanuda exactamente esas sesiones. Las que ya estaban suspendidas siguen así.

Los builds de desarrollo (`dev`) nunca muestran el aviso, ni tampoco un daemon más nuevo que la app.

## Icono

Las fuentes están en `assets/icon/`:

- `panal-icon.svg`: el icono cuadrado, a sangre completa;
- `panal-icon-rounded.svg`: con esquinas redondeadas (el del README);
- `panal-icon-macos.svg`: con el margen de la plantilla de iconos de macOS, que es el que usa la
  app.

Wails genera el icono del `.app` a partir de `app/build/appicon.png` (1024×1024). Para regenerarlo
con Inkscape 1.x:

```sh
inkscape assets/icon/panal-icon-macos.svg \
  --export-type=png --export-filename=app/build/appicon.png -w 1024 -h 1024
```

En macOS la CLI de Inkscape no queda en el `PATH`; está en
`/Applications/Inkscape.app/Contents/MacOS/inkscape`. Usa colores sólidos en los bordes (no
`stroke-opacity`): los trazos semitransparentes se suman donde se cruzan.

## Limitaciones conocidas

- **Primero macOS.** La app se prueba sobre todo en macOS (Apple Silicon); la app de Linux es
  experimental (abajo). El transporte es un socket unix; no hay soporte para Windows.
- **Actualizar no reinicia el daemon**: tras instalar una versión nueva, un daemon que ya estaba
  corriendo sigue con el código viejo hasta `panal stop` (o un reinicio de la computadora).
- La **app de Linux** (AppImage, .deb) es experimental: el CI la compila, prueba y empaqueta en
  Ubuntu 24.04 (`scripts/package-linux.sh`), pero está poco probada en escritorios reales. Cada
  cambio corre los tests del daemon y la CLI en Ubuntu (`linux` de ci.yml, sin GTK); los tests y
  el empaquetado de la app (`linux-app.yml`, con GTK/WebKitGTK) corren en main y en los pull
  requests que tocan `app/`, `packaging/`, los workflows o las dependencias.
- **Un reinicio del daemon (o de la computadora) mata los procesos.** Las sesiones vuelven como
  `suspended`, con su scrollback, y se relanzan al reanudarlas: Claude Code y codex continúan su
  conversación por id, opencode y Cursor CLI con `--continue` y Hermes con `-c` (la *última* sesión de cada
  proyecto, que puede no ser la misma si hay varias en una carpeta), otros agentes solo con
  `resume_args`. Una shell vuelve en su carpeta inicial, sin lo que estuviera corriendo en ella. codex solo informa su thread id
  tras el primer turno, así que una sesión de codex que nunca terminó un turno empieza de cero.
- **Persistir el scrollback cuesta escrituras en disco**: se reescribe el ring entero (hasta 8 MiB)
  como mucho cada 2 s por sesión mientras produce salida.
- **Notificaciones con la app cerrada**: si el daemon no está dentro del `.app` (lo arrancó una CLI
  instalada fuera del bundle) o el bundle no tiene permiso, las notificaciones no se muestran
  (`daemon.log` dice por qué). Con `notify.fallback: osascript` se envían con `osascript`: aparecen
  como *Editor de Scripts* y al pulsarlas se abre Editor de Scripts, no panal.
- **codex** no avisa cuando empieza a trabajar: "trabajando" lo decide la heurística y no hay
  aviso de «¿trabado?».
- **«¿Trabado?»** solo existe en sesiones con hooks (Claude Code), y como el spinner de Claude
  redibuja cada segundo, detecta sobre todo cuelgues completos, no herramientas lentas.
- **Línea de actividad en TUIs** que redibujan la pantalla (opencode, codex…): es la última línea
  que pintaron, que no siempre es la más informativa.
- **Kiro CLI**: el nombre del binario (`kiro-cli chat`) viene de la documentación y no se ha
  verificado con una instalación real; se puede corregir con `cmd:` en `profiles.yaml`.
- **Hermes Agent** se lanza como `hermes --cli` y solo desde el menú **+** cuando el binario está
  instalado y el perfil sigue activado. La línea de aprobación `[o]nce | [s]ession` marca la
  sesión como esperando. `args: ["--tui"]` o `args: []` sustituye ese default; `resume_args: []`
  omite el reenganche con `-c`; `enabled: false` lo oculta. Cerrar la terminal desconecta del
  gateway de Hermes (el turno sigue hasta `/exit` o Ctrl+D). panal no escribe en `~/.hermes`.
- **`profiles.yaml` pierde los comentarios** cuando la app lo guarda (se reescribe en formato
  canónico). `settings.yaml` sí los conserva.
- El merge de un worktree **no hace stash**: la carpeta principal tiene que estar limpia y en la
  rama base.
