#!/bin/sh
# Skill Forge installer — POSIX sh, no dependencies beyond curl/tar.
#
#   curl -fsSL https://raw.githubusercontent.com/mrdulasolutions/skillforge/main/install.sh | sh
#
# Environment overrides:
#   SKILLFORGE_VERSION   pin a version (default: latest release)
#   SKILLFORGE_BIN_DIR   install dir (default: ~/.local/bin, else /usr/local/bin)
#   SKILLFORGE_ALIAS     also create this alias symlink (e.g. "skill" or "forge")
#   SKILLFORGE_LOCAL_BIN install this local binary instead of downloading (testing)
#   SKILLFORGE_DRY_RUN   print actions without installing
#   NO_COLOR             disable color

set -eu

OWNER="mrdulasolutions"
REPO="skillforge"
BIN="skillforge"

# --- colors -----------------------------------------------------------------
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  ESC=$(printf '\033')
  RESET="${ESC}[0m"; BOLD="${ESC}[1m"; DIM="${ESC}[2m"
  GREEN="${ESC}[1;32m"; RED="${ESC}[1;31m"; YELLOW="${ESC}[1;33m"; CYAN="${ESC}[36m"
  TRUECOLOR=1
else
  ESC=""; RESET=""; BOLD=""; DIM=""; GREEN=""; RED=""; YELLOW=""; CYAN=""; TRUECOLOR=0
fi

# gradient_ascii prints an ASCII string with a left-to-right fire gradient.
gradient_ascii() {
  text="$1"
  len=$(printf '%s' "$text" | wc -c | tr -d ' ')
  if [ "$TRUECOLOR" != "1" ] || [ "$len" -lt 1 ]; then printf '%s' "$text"; return; fi
  i=0
  while [ "$i" -lt "$len" ]; do
    ch=$(printf '%s' "$text" | cut -c $((i + 1)))
    if [ "$len" -gt 1 ]; then t=$((i * 1000 / (len - 1))); else t=0; fi
    g=$((61 + (196 - 61) * t / 1000))
    printf '%s[1;38;2;255;%d;0m%s' "$ESC" "$g" "$ch"
    i=$((i + 1))
  done
  printf '%s' "$RESET"
}

ok()   { printf '%s✓%s %s\n' "$GREEN" "$RESET" "$1"; }
err()  { printf '%s✗%s %s\n' "$RED" "$RESET" "$1" >&2; }
warn() { printf '%s▲%s %s\n' "$YELLOW" "$RESET" "$1"; }
step() { printf '%s→%s %s\n' "$CYAN" "$RESET" "$1"; }

banner() {
  printf '\n'
  printf '  %s%s%s\n' "$BOLD" "$(gradient_ascii 'SKILL FORGE')" "$RESET"
  printf '  %s✦ forge portable agentic skills & plugins%s\n\n' "$DIM" "$RESET"
}

die() { err "$1"; exit 1; }

# --- detect platform --------------------------------------------------------
detect_platform() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)
  case "$os" in
    linux|darwin) ;;
    *) die "unsupported OS: $os (Linux and macOS are supported; use install.ps1 on Windows)" ;;
  esac
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) die "unsupported architecture: $arch" ;;
  esac
  PLATFORM_OS="$os"; PLATFORM_ARCH="$arch"
}

have() { command -v "$1" >/dev/null 2>&1; }

fetch() { # fetch URL -> stdout
  if have curl; then curl -fsSL "$1"
  elif have wget; then wget -qO- "$1"
  else die "need curl or wget"; fi
}

download() { # download URL DEST
  if have curl; then curl -fsSL "$1" -o "$2"
  elif have wget; then wget -qO "$2" "$1"
  else die "need curl or wget"; fi
}

resolve_version() {
  if [ -n "${SKILLFORGE_VERSION:-}" ]; then VERSION="$SKILLFORGE_VERSION"; return; fi
  api="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
  VERSION=$(fetch "$api" 2>/dev/null | grep -m1 '"tag_name"' | sed 's/.*"tag_name":[ ]*"\([^"]*\)".*/\1/' || true)
  [ -n "$VERSION" ] || die "could not resolve latest version (set SKILLFORGE_VERSION)"
}

choose_bin_dir() {
  if [ -n "${SKILLFORGE_BIN_DIR:-}" ]; then BIN_DIR="$SKILLFORGE_BIN_DIR"; return; fi
  if [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
    BIN_DIR="$HOME/.local/bin"
  else
    BIN_DIR="/usr/local/bin"
  fi
}

install_binary() { # install_binary SRC
  src="$1"
  mkdir -p "$BIN_DIR" 2>/dev/null || true
  dest="$BIN_DIR/$BIN"
  if [ -w "$BIN_DIR" ]; then
    install -m 0755 "$src" "$dest"
  else
    warn "elevating with sudo to write $BIN_DIR"
    sudo install -m 0755 "$src" "$dest"
  fi
  ok "installed ${BOLD}$dest${RESET}"
  if [ -n "${SKILLFORGE_ALIAS:-}" ]; then
    ln -sf "$dest" "$BIN_DIR/$SKILLFORGE_ALIAS" 2>/dev/null \
      && ok "alias ${BOLD}$SKILLFORGE_ALIAS${RESET} → $BIN" \
      || warn "could not create alias $SKILLFORGE_ALIAS"
  fi
}

ensure_path() {
  case ":$PATH:" in
    *":$BIN_DIR:"*) return ;;
  esac
  warn "$BIN_DIR is not on your PATH"
  rc=""
  case "${SHELL:-}" in
    *zsh) rc="$HOME/.zshrc" ;;
    *bash) rc="$HOME/.bashrc" ;;
  esac
  line="export PATH=\"$BIN_DIR:\$PATH\""
  if [ -n "$rc" ]; then
    printf '\n# Added by Skill Forge installer\n%s\n' "$line" >> "$rc"
    step "added to $rc — run: ${BOLD}source $rc${RESET}"
  else
    step "add this to your shell profile: ${BOLD}$line${RESET}"
  fi
}

main() {
  banner
  detect_platform
  step "platform: ${BOLD}${PLATFORM_OS}/${PLATFORM_ARCH}${RESET}"
  choose_bin_dir

  tmp=$(mktemp -d 2>/dev/null || mktemp -d -t skillforge)
  trap 'rm -rf "$tmp"' EXIT

  if [ -n "${SKILLFORGE_LOCAL_BIN:-}" ]; then
    step "using local binary: $SKILLFORGE_LOCAL_BIN"
    [ "${SKILLFORGE_DRY_RUN:-}" = "1" ] && { ok "dry-run: would install to $BIN_DIR/$BIN"; return; }
    install_binary "$SKILLFORGE_LOCAL_BIN"
  else
    resolve_version
    step "version:  ${BOLD}${VERSION}${RESET}"
    asset="${BIN}_${VERSION#v}_${PLATFORM_OS}_${PLATFORM_ARCH}.tar.gz"
    base="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}"
    url="${base}/${asset}"
    if [ "${SKILLFORGE_DRY_RUN:-}" = "1" ]; then
      ok "dry-run: would download ${url}"
      ok "dry-run: would install to $BIN_DIR/$BIN"
      return
    fi
    step "downloading ${asset}"
    download "$url" "$tmp/$asset" || die "download failed: $url"
    # Optional checksum verification (sha256sum on Linux, shasum on macOS).
    if download "${base}/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
      sumcmd=""
      if have sha256sum; then sumcmd="sha256sum -c -"
      elif have shasum; then sumcmd="shasum -a 256 -c -"; fi
      if [ -n "$sumcmd" ]; then
        (cd "$tmp" && grep " $asset\$" checksums.txt | $sumcmd >/dev/null 2>&1) \
          && ok "checksum verified" || warn "checksum not verified"
      fi
    fi
    tar -xzf "$tmp/$asset" -C "$tmp" || die "extract failed"
    install_binary "$tmp/$BIN"
  fi

  ensure_path
  printf '\n'
  ok "Skill Forge is ready. Try: ${BOLD}${BIN}${RESET} or ${BOLD}${BIN} help${RESET} for commands"
  printf '\n'
}

main "$@"
