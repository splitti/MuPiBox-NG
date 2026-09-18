# local-ai MCP-Server

Bindet Ollama/`qwen3:8b` als MCP-Tool `local_ai` für Claude Code ein (siehe CLAUDE.md,
Abschnitt „DEV/TEST-Rollenverteilung“). Einmalige Einrichtung pro Checkout:

```sh
cd .claude/tools/local-ai-mcp
python3 -m venv venv
./venv/bin/pip install "mcp<2"
```

Danach `claude mcp list` prüfen (Status sollte „Connected“ zeigen) und die Session neu starten,
damit das Tool geladen wird. Konfiguration über Umgebungsvariablen, siehe Kopfkommentar in
`server.py`.
