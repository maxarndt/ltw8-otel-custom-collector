# knxreceiver — KNX Receiver für OpenTelemetry Collector

## Projektname & Namenskonvention

| Ebene | Name |
|---|---|
| Go-Package / Verzeichnis | `knxreceiver` |
| YAML-Key im Collector | `knx` |
| Go-Module (`go.mod`) | `github.com/{username}/knxreceiver` |
| GitHub-Repository | `knxreceiver` |

Folgt exakt der Contrib-Konvention (`snmpreceiver` → `snmp`, `netflowreceiver` → `netflow`).
Contrib-PR-fähig ohne Umbenennung.

## Ziel

Implementierung eines **custom KNX Receivers** für den OpenTelemetry Collector in Go.
Der Receiver liest Energiemessdaten (und weitere Metriken) vom KNX-Bus und liefert sie
als OTEL-Metriken weiter — von wo sie per `prometheusremotewrite`-Exporter in
**VictoriaMetrics** geschrieben werden.

Der Receiver bleibt ein **privater Custom Collector** (kein Contrib-PR geplant).

---

## Infrastruktur

| Komponente | Details |
|---|---|
| KNX Gateway | MDT IP Router (KNX/IP, REG-Bauform) |
| KNX Aktoren | MDT AMS Schaltaktoren mit Strommessung (Wh-Aufsummierung pro Kanal) |
| Metrics Backend | VictoriaMetrics (bereits laufend, remote_write-kompatibel) |
| OTEL Stack | Bestehender OTEL Monitoring Stack |
| Deployment | Kubernetes (kein Helm — plain YAML Manifeste) |
| Entwicklung | Go, Visual Studio Code + Claude Code |

---

## Architekturentscheidung

```
KNX Bus (TP)
    │
    └─► MDT IP Router (KNX/IP)
              │
              │  KNX/IP Tunnel (Unicast) oder Multicast — noch offen,
              │  Konfiguration soll per YAML wählbar sein
              ▼
    ┌─────────────────────────────┐
    │   Custom OTEL Collector     │
    │   ┌─────────────────────┐   │
    │   │   knxreceiver       │   │  ← dieses Projekt
    │   │   (Go Modul)        │   │
    │   └────────┬────────────┘   │
    │            │ pmetric        │
    │   ┌────────▼────────────┐   │
    │   │ prometheusremotewrite│  │
    │   │ exporter            │   │
    │   └─────────────────────┘   │
    └─────────────────────────────┘
              │
              │  HTTP remote_write
              ▼
    VictoriaMetrics
```

Der Custom Collector wird mit dem **OpenTelemetry Collector Builder (ocb)** gebaut,
als Docker-Image verpackt und per Kubernetes Deployment deployed.

---

## Referenzimplementierung

Basis für den KNX-Teil: **`chr-fritz/knx-exporter`** (Go, MIT-Lizenz)
- Repo: https://github.com/chr-fritz/knx-exporter
- Relevante Pakete zum Portieren/Wiederverwenden:
  - KNX/IP-Verbindung via `vapourismo/knx-go`
  - DPT-Dekodierung (alle relevanten Datentypen)
  - GroupAddress-Konfigurationsschema (YAML)
  - `ReadStartup`-Logik (einmaliges Lesen beim Start)

**Nicht übernehmen:** Prometheus-Exporter-Teil (`prometheus.Gauge`/`Counter`) —
wird durch OTEL `pmetric`-API ersetzt.

---

## Zu erfassende KNX Datenpunkte

Alle Typen werden per YAML konfiguriert — der Receiver soll generisch sein und beliebige
Gruppenadressen/DPTs unterstützen. Priorität der DPT-Implementierung:

**Phase 1 — Energie & Leistung (MDT AMS Aktoren):**
- `DPT 13.010` — Energie, Wh-Counter (aufsummiert pro Kanal) → OTEL `Sum`, monoton
- `DPT 14.056` — Wirkleistung, W (aktuelle Last pro Kanal) → OTEL `Gauge`

**Phase 2 — Weitere Typen (generisch erweiterbar):**
- `DPT 9.001` — Temperatur (°C) → `Gauge`
- `DPT 9.004` — Beleuchtungsstärke (Lux) → `Gauge`
- `DPT 9.007` — Relative Luftfeuchte (%) → `Gauge`
- `DPT 1.001` — Binär Schalten (An/Aus) → `Gauge` (0.0 / 1.0)
- `DPT 5.001` — Prozentwert 0–100% → `Gauge`
- `DPT 5.010` — Ganzzahl 0–255 → `Gauge`

**Architektur-Anforderung:** Der DPT-Decoder soll als eigene Schicht implementiert werden
(`dpt_decoder.go`), die einfach um neue DPTs erweiterbar ist. Unbekannte DPTs werden
geloggt und übersprungen (kein Absturz).

---

## Bus-Last Strategie

- **`ReadStartup: true`** — einmaliges GroupValueRead pro konfigurierter Gruppenadresse
  beim Start (Initialwert sofort verfügbar). Interval: 200ms zwischen Reads (Default
  aus knx-exporter, beibehalten).
- **`ReadActive: false`** — kein zyklisches Polling im Dauerbetrieb.
- **Normalfall: passives Lauschen auf `GroupValueWrite`** — die MDT AMS Aktoren senden
  Wh-Werte zyklisch von sich aus (konfiguriert in ETS, z.B. alle 5 Minuten).
  Erzeugt null zusätzliche Bus-Last.
- Begründung: KNX TP Bus hat ~50 Telegramme/s Maximum. Zyklisches Polling wäre
  unnötig und erzeugt 2 Telegramme pro Kanal pro Read (Read + Response).

---

## Versionen & Toolchain

| Tool | Version |
|---|---|
| Go | **1.25** (vorgeschrieben durch OCB Builder ab v0.148.0) |
| OTEL Collector Core | **v1.55.0** |
| OTEL Collector Contrib | **v0.149.0** |
| ocb (Collector Builder) | **v0.149.0** (immer gleiche Version wie Collector) |
| knx-go | `vapourismo/knx-go` (aktuelle Version von main) |

> **Hinweis Versionskopplung:** OCB-Version, Collector-Core und Contrib müssen exakt
> übereinstimmen. Bei einem Update immer alle drei gleichzeitig anheben.

---

## Projektstruktur (angestrebte Verzeichnisstruktur)

```
knxreceiver/
├── CLAUDE.md                  ← diese Datei
├── receiver/
│   └── knxreceiver/
│       ├── config.go          ← Config-Struct + Validate()
│       ├── factory.go         ← receiver.Factory, NewFactory()
│       ├── receiver.go        ← Start(), Shutdown(), KNX-Hauptloop, Reconnect
│       ├── knx_client.go      ← Wrapper um vapourismo/knx-go (Tunnel + Router)
│       ├── dpt_decoder.go     ← DPT-Dekodierung: []byte → float64 + MetricType
│       ├── converter.go       ← KNX GroupEvent → pmetric.Metrics
│       └── config_example.yaml← Beispielkonfiguration mit allen DPT-Typen
├── collector/
│   ├── builder-config.yaml    ← ocb Manifest (Go 1.25, v0.149.0)
│   └── collector-config.yaml  ← Collector YAML (receivers/exporters/pipelines)
├── deploy/
│   ├── namespace.yaml
│   ├── deployment.yaml        ← 1 Replica, ConfigMap-Mount, Liveness/Readiness
│   └── service.yaml           ← Health-Check Port 13133 (intern)
├── Makefile                   ← deploy-Target: ConfigMap aus collector-config.yaml + kubectl apply
├── Dockerfile                 ← Multi-stage: ocb build → distroless run
└── go.mod                     ← go 1.25
```

---

## Receiver Interface (OTEL Pflicht)

Der Receiver muss folgende Interfaces implementieren:

```go
// component.Component
Start(ctx context.Context, host component.Host) error
Shutdown(ctx context.Context) error

// receiver.Metrics (via consumer.Metrics)
// Daten werden aktiv per nextConsumer.ConsumeMetrics() gepusht — kein Pull
```

Factory-Registrierung via `receiver.NewFactory(...)` mit
`receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelAlpha)`.

---

## KNX Verbindungsmodi

Beide Modi werden unterstützt — der Mehraufwand ist minimal da `vapourismo/knx-go`
beide als `GroupTunnel` bzw. `GroupRouter` anbietet. Der Switch liegt in einer Zeile
der YAML-Konfiguration.

| | Tunnel (Unicast) | Router (Multicast) |
|---|---|---|
| Verbindung | Punkt-zu-Punkt auf MDT IP Router | Multicast-Gruppe `224.0.23.12:3671` |
| Verbindungsslots | Belegt 1 von 4 Slots am MDT Router | Keiner — unbegrenzte Teilnehmer |
| NAT / Subnetz | Funktioniert problemlos | Nur im gleichen L2-Segment (ohne Multicast-Routing) |
| K8s Pod-Restart | Neuer Tunnel-Handshake nötig | Sofort — kein Handshake |
| GroupValueRead | Vollständig unterstützt | Möglich, aber unüblich |
| Netzwerk-Anforderung | Keine | IGMP Snooping korrekt konfiguriert |

**Empfehlung für dieses Projekt:** Router-Modus als Default, da der Receiver
primär passiv lauscht (`ReadActive: false`) und kein Verbindungsslot am MDT IP
Router belegt werden soll. Tunnel als Alternative für Umgebungen ohne Multicast.



```yaml
knx:
  connection:
    # Router-Modus (Default — empfohlen für rein-lesende Receiver)
    type: "router"
    multicast_address: "224.0.23.12:3671"
    # Tunnel-Modus (Alternative, falls kein Multicast verfügbar)
    # type: "tunnel"
    # endpoint: "192.168.1.x:3671"
  read_startup_interval: 200ms
  address_configs:
    "1/1/1":
      name: "strom_eg_kueche_wh"
      dpt: "13.010"
      export: true
      metric_type: "sum"     # Wh-Counter → OTEL Sum (monoton, kumulativ)
      read_startup: true
      labels:
        room: "kueche"
        floor: "eg"
    "1/1/2":
      name: "leistung_eg_kueche_w"
      dpt: "14.056"
      export: true
      metric_type: "gauge"   # Watt → OTEL Gauge
      read_startup: true
      labels:
        room: "kueche"
        floor: "eg"
```

---

## pmetric Mapping

| KNX DPT | Beschreibung | OTEL Metric Type | Temporality | Begründung |
|---|---|---|---|---|
| 13.010 | Energie (Wh) | `Sum` | Kumulativ, monoton | Zähler, läuft nur hoch |
| 14.056 | Wirkleistung (W) | `Gauge` | — | Momentanwert |
| 9.001 | Temperatur (°C) | `Gauge` | — | Momentanwert |
| 9.004 | Beleuchtung (Lux) | `Gauge` | — | Momentanwert |
| 9.007 | Luftfeuchte (%) | `Gauge` | — | Momentanwert |
| 1.001 | Binär (An/Aus) | `Gauge` | — | 0.0=Aus, 1.0=An |
| 5.001 | Prozent (0–100%) | `Gauge` | — | Momentanwert |
| 5.010 | Ganzzahl (0–255) | `Gauge` | — | Momentanwert |

Ressource-Attribute: `knx.physical_address` (physikalische Adresse des Receivers am Bus)
Metric-Attribute (Labels): aus `address_configs[].labels` + `knx.group_address`

---

## Kubernetes Deployment (plain YAML, kein Helm)

- Namespace: `monitoring` (oder bestehender OTEL-Namespace)
- 1 Replica (KNX Tunnel erlaubt nur eine Verbindung gleichzeitig — kein horizontales Scaling)
- ConfigMap für `collector-config.yaml`
- Secret oder ConfigMap für KNX-Endpoint (je nach Netzwerktopologie)
- Kein `hostNetwork` nötig — KNX/IP über normales Pod-Netzwerk erreichbar,
  solange MDT IP Router im selben L2-Segment oder via Unicast-Tunnel erreichbar ist
- Liveness/Readiness Probe auf OTEL Health-Check Extension (`1777/tcp`)

---

## Offene Entscheidungen (beim Start der Implementierung klären)

1. **KNX Connection-Typ**: Tunnel (Unicast, stabiler, 1 Verbindung) vs. Router
   (Multicast, kein explizites Pairing nötig). Empfehlung: Tunnel als Default,
   Router als Alternative in Config.
2. **Fehlerbehandlung bei KNX-Verbindungsabbruch**: Reconnect-Loop mit Backoff
   im `receiver.go`, oder Collector-Restart via K8s?
   Empfehlung: Reconnect-Loop (vermeidet unnötige Pod-Restarts).
3. **Startup-Read Timeout**: Was passiert wenn ein Gerät beim Startup nicht antwortet?
   Empfehlung: Timeout pro Gruppenadresse, Fehler loggen, weitermachen.
