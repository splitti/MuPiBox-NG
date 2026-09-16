# Repository audit – 2026-09-16

> English. German version: [repository-audit.md](repository-audit.md)

Audited starting point: `main` at `598c92c59ca18e19fef80b4f8c7587ecf6f0839d` (2026-02-10, “UI update”). GitHub read/write access was confirmed; branch `rebuild/go-foundation` was created from exactly that commit. `main` was not overwritten.

The original tree had no README, AGENTS.md or tests/CI. It used Go 1.19, module `mupibox`, with no external Go dependencies. It contained cmd, catalog/player/state/status, a simple web UI with fonts/icons, two installer scripts and one schema.

| Component | Finding | Decision |
| --- | --- | --- |
| Go structure/module | Small dependency-free base | Keep the structural idea and module name; target Go 1.24 |
| `webui/webui.go` | Embed handler existed but main used filesystem assets | Use the embedded approach actively |
| Player | MemoryPlayer with demo timings/titles, no audio output | Replace with controller + replaceable audio adapter |
| HTTP API | Many methods without error paths; collections hard-coded | Validated JSON commands tied to the real library |
| UI | Fixed 800×480; tiles only logged to console; volume was local-only | Responsive UI, real commands and shared status |
| Catalog | Spotify/Amazon/RSS only example IDs/URLs | Archive as design reference; do not advertise unsupported providers |
| Source resolver | Reported every known source as available without verification | Do not reuse; avoid false availability claims |
| Progress store | Useful model, but ignored read/JSON/write errors and was not atomic | Reuse the concept only; implement robust persistence separately |
| Hardware status | Fixed int/bool fields without an “unknown” state | Do not invent hardware values; add capabilities/nullable telemetry later |
| Installer | Global apt upgrade, root service, mixed program/data | Archive; use a constrained systemd template with a dedicated user |
| Fonts/covers/icons | Only icon note under LICENSES; origin of other assets unclear | Preserve in archive; active UI uses system fonts/CSS |

The complete prototype including assets remains byte-identical under `legacy/prototype/`. Its own go.mod isolates it from the new build. The existing LICENSES file remains. No new project license was invented; project licensing and asset rights must be clarified before distribution. Old installers are historical references and must not be executed for the new application.

## Previous MuPiBox

The current reference autosetup script was reviewed again. Node.js/PM2, Python hardware packages, MPlayer and librespot are present, along with release selection via version.json. The script makes broad system changes and copies/replaces configuration and program files. These changes and the installer are not reused without review. No hardware pinout was inferred merely from package names.

## Development environment

MuPiBox Dev currently provides only `list_files`, `read_file`, `write_file`, `go_check`, `git_status` and `git_diff`. Go 1.24.4 linux/amd64 was confirmed. The initial state contained only `dev-access-check.txt` and reported “not a git repository”. A later test failure revealed the actual project path `/opt/mupibox-ng`; the new sources are now built and tested there.

The separate ChatGPT checkout was cloned through Git and is not the LXC. GitHub access does not replace Git initialization/linking inside the LXC. `scripts/link-lxc.sh` prepares that reconciliation without overwriting divergent local files; because no shell action is exposed through MuPiBox Dev, splitti must run it once when needed. Tests must not misuse Go actions to clone repositories or execute Git.
