# AC-6 Deploy Verification Checklist

> **Note:** Live Render deploy verification deferred. This checklist supports local verification.
> Update `RENDER_URL` to the live URL when deployed.

## Prerequisites

```bash
export PORT="${PORT:-8080}"
export RENDER_URL="http://localhost:${PORT}"
```

## P1: Health Check

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:${PORT}/health
# Expected: 200
```

Verify JSON response:
```bash
curl -s http://localhost:${PORT}/health | python3 -m json.tool
# Expected: {"status": "ok"}
```

## P2: Theme Presence

```bash
curl -s http://localhost:${PORT}/ | grep -o 'href="/static/css/space.css"'
# Expected: href="/static/css/space.css"
```

```bash
curl -s http://localhost:${PORT}/ | grep -o 'class="neon-'
# Expected: at least 1 match (neon-button, neon-gold, etc.)
```

Check at least 2 space-theme markers:
```bash
curl -s http://localhost:${PORT}/ | grep -o -E 'class="(neon-|movie-hero|bracket-connector)"'
# Expected: >= 2 matches
```

## P3: Space CSS Served via embed.FS

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:${PORT}/static/css/space.css
# Expected: 200
```

```bash
curl -s -I http://localhost:${PORT}/static/css/space.css | grep -i "content-type"
# Expected: Content-Type: text/css
```

## P4: Responsive Breakpoint

```bash
curl -s http://localhost:${PORT}/static/css/space.css | grep -c "@media (max-width: 1024px)"
# Expected: >= 1
```

## P5: .movie-hero Rule

```bash
curl -s http://localhost:${PORT}/static/css/space.css | grep -A5 "\.movie-hero"
# Expected: Shows font-size, color, text-shadow properties
```

## P6: CSS Size, @keyframes, No Images

```bash
curl -s http://localhost:${PORT}/static/css/space.css | wc -c
# Expected: <= 153600 (150KB)
```

```bash
curl -s http://localhost:${PORT}/static/css/space.css | grep -c "@keyframes"
# Expected: >= 1
```

```bash
curl -s http://localhost:${PORT}/static/css/space.css | grep -E 'url\(.*\.(gif|png|jpg|jpeg|webp|mp4|webm)\)'
# Expected: no output (empty)
```

## P7: Battle Endpoint Works

```bash
# Create a session first
SESSION_ID=$(curl -s -c - http://localhost:${PORT}/ | grep bbw_session | awk '{print $NF}')
echo "Session: $SESSION_ID"
```

```bash
# Add options to wheels 0 and 1
curl -s -b "bbw_session=${SESSION_ID}" -X POST -d "text=MovieA" http://localhost:${PORT}/wheel/0/option > /dev/null
curl -s -b "bbw_session=${SESSION_ID}" -X POST -d "text=MovieB" http://localhost:${PORT}/wheel/1/option > /dev/null
```

```bash
# POST /battle/qf1
curl -s -o /dev/null -w "%{http_code}" -b "bbw_session=${SESSION_ID}" -X POST http://localhost:${PORT}/battle/qf1
# Expected: 200
```

## P8: render.yaml Config

```bash
grep -c "startCommand:" render.yaml
# Expected: 1
grep -c "PORT" render.yaml
# Expected: 1
grep -c "healthCheckPath" render.yaml
# Expected: 1
```

## Rodney Browser Screenshots

### Desktop (1920px)

```bash
uvx rodney start
uvx rodney open http://localhost:${PORT}/
# Verify: starfield visible, cosmic panels, neon text, 4-column bracket
uvx rodney screenshot docs/evidence/6/theme-desktop.png
```

### Tablet (768px)

If rodney supports viewport resize:
```bash
# Set viewport to 768px width
uvx rodney eval "window.resizeTo(768, 1024);"
uvx rodney screenshot docs/evidence/6/theme-tablet.png
# Expected: bracket stacks/condenses, no horizontal scrollbar
```

If not, open browser manually at 768px and capture.

### Hero Result

```bash
# Add options to all 8 wheels and run full bracket
for i in $(seq 0 7); do
    curl -s -b "bbw_session=${SESSION_ID}" -X POST -d "text=Opt${i}" "http://localhost:${PORT}/wheel/${i}/option" > /dev/null
done
for match in qf1 qf2 qf3 qf4 sf1 sf2 final; do
    curl -s -b "bbw_session=${SESSION_ID}" -X POST "http://localhost:${PORT}/battle/${match}" > /dev/null
done
```

```bash
uvx rodney open http://localhost:${PORT}/
uvx rodney screenshot docs/evidence/6/hero-result.png
# Expected: Shows "You're watching: <movie>" with neon gold styling
```

```bash
uvx rodney stop
```
