# Distribution — Homebrew + GitHub Releases

> Cómo llegar a `brew install <user>/tap/timer` funcionando con un solo `git tag v0.1.0 && git push --tags`.

## Stack de distribución

- **GoReleaser** — build multi-arch + archives + changelog + publicación.
- **GitHub Releases** — hosting de los binarios.
- **Homebrew tap propio** — repo `homebrew-tap` con Formulas generadas por GoReleaser.
- **GitHub Actions** — dispara goreleaser al taggear.

## Qué se publica

Targets:
- `darwin/amd64` (Mac Intel)
- `darwin/arm64` (Mac Apple Silicon)
- `linux/amd64`
- `linux/arm64`
- (Windows si hay ganas — no prioritario para v1)

Artefactos por target:
- `timer_v0.1.0_darwin_arm64.tar.gz` con `timer` + `README.md` + `LICENSE`
- SHA256 checksums
- Changelog auto-generado desde conventional commits

## `.goreleaser.yaml` base

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: timer
    main: ./cmd/timer
    binary: timer
    env:
      - CGO_ENABLED=0          # pure-Go sqlite driver (modernc.org/sqlite)
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w
      - -X timer/internal/version.Version={{.Version}}
      - -X timer/internal/version.Commit={{.Commit}}
      - -X timer/internal/version.Date={{.Date}}

archives:
  - format: tar.gz
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
    files:
      - README.md
      - LICENSE

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
  groups:
    - title: Features
      regexp: "^feat"
      order: 0
    - title: Bug fixes
      regexp: "^fix"
      order: 1
    - title: Other
      order: 99

brews:
  - name: timer
    repository:
      owner: <user>
      name: homebrew-tap
    homepage: "https://github.com/<user>/timer-cli"
    description: "Local-first time tracker with CLI, TUI and MCP"
    license: "MIT"
    install: |
      bin.install "timer"
    test: |
      system "#{bin}/timer version"
```

## GitHub Action

`.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0     # necesario para changelog

      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          # Token con acceso al repo homebrew-tap
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

### Token para el tap

El `GITHUB_TOKEN` default sirve para publicar releases en el repo mismo. Para pushear al tap (repo separado) hace falta un **Personal Access Token** con scope `repo`, guardado como secret `HOMEBREW_TAP_TOKEN`.

## Repo del tap

Repo público `homebrew-tap`, estructura:

```
homebrew-tap/
└── Formula/
    └── timer.rb       # generado por goreleaser
```

GoReleaser crea el PR/commit con el Formula actualizado cada release. No se edita a mano.

## Cómo instala el usuario

```bash
brew tap <user>/tap
brew install timer
```

O en un comando:
```bash
brew install <user>/tap/timer
```

## Proceso de release manual

Una vez armado todo:

```bash
# verificar que main está verde
git checkout main && git pull

# taggear
git tag -a v0.1.0 -m "v0.1.0: first public release"
git push origin v0.1.0

# GitHub Actions arranca, 2-3 min después está el binario en releases y el tap actualizado
```

## CI previa al release

Antes de taggear, CI debería validar (`ci.yml`, trigger en PRs y push a main):

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - run: go mod download
      - run: go vet ./...
      - run: go test ./... -race -cover

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  goreleaser-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: check
```

## Install script (opcional)

Para usuarios sin brew (Linux server, etc.):

```bash
#!/usr/bin/env sh
set -e
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
esac
VERSION=$(curl -s https://api.github.com/repos/<user>/timer-cli/releases/latest | grep tag_name | cut -d '"' -f 4)
URL="https://github.com/<user>/timer-cli/releases/download/${VERSION}/timer_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -L "$URL" | tar xz -C /usr/local/bin timer
echo "timer ${VERSION} installed"
```

Servir desde una página simple o GitHub Pages. Opcional.

## Versionado

- **v0.x.x** — pre-stable, breaking changes permitidos. Mientras se prueba el modelo con los primeros usuarios.
- **v1.0.0** — API pública (comandos CLI, schema SQLite) estable. Breaking change = v2.0.0.

Conventional commits para el changelog:
- `feat:` — minor bump (0.1.0 → 0.2.0)
- `fix:` — patch bump (0.1.0 → 0.1.1)
- `feat!:` o `BREAKING CHANGE:` — major bump

Automatizable con [`svu`](https://github.com/caarlos0/svu) si querés evitar decidir versiones a mano.

## Signing (opcional, para reducir fricción en macOS)

Sin firma, macOS muestra "cannot be opened because the developer cannot be verified" la primera vez. Opciones:

1. **No firmar** — documentar en README el workaround (`xattr -d com.apple.quarantine timer`).
2. **Firmar con Developer ID** — requiere cuenta Apple Developer (99 USD/año). GoReleaser lo soporta con `notarize`.
3. **Distribuir solo por brew** — brew resuelve el quarantine, el problema desaparece.

Para v1 elegir (1) o (3). Firmar cuando haya usuarios reales pidiéndolo.
