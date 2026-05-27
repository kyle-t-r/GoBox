# GoBox
Event driven server monitoring tool written in Go, including two event publisher utility projects.

### GoBox Utility
Exposes a single Gin endpoint at `/read` to view events with `limit`, `offset`, and `level` query params.

The main utility will ingest a `config.yaml` file, and request the following values: 
- Database:
  - Type: `sqlite3` or `mysql`
  - Connection: SQLite `.db` file or MySQL auth URL
- Server:
  - Port: `8080` by default, this will be the exposed port for report view
- Publishers:
  - Name for publisher, this will be used in the logs to identify the process
    - Schedule, a CRON expression ([learn more](https://pkg.go.dev/github.com/robfig/cron))
    - Executable to run
    - Config values, which will be passed to the child process as a JSON payload

**Example Config**
```
database:
  type: sqlite3
  connection: "./sqlite.db"

server:
  port: ":8080"

publishers:
  sys-monitor:
    schedule: "@every 1m"
    executable: "./publishers/sys-monitor/sys-monitor"
    config:
      cpu_warn: 50.0
      cpu_critical: 90.0
      mem_warn: 50.0
      mem_critical: 90.0
```

### Publishers

Publishers must use the `publib` module to publish events, or modify the database directly. When inserting directly, the publisher has to follow the schema created by the GoBox utility.

> [!NOTE]
> Example Query: `INSERT INTO events (level, time, service, message) VALUES (?, ?, ?, ?)`

**disk-monitor**
Reports the disk utilization for the given label or mount path.

Config:
  - `disk_label` (String)
  - `disk_warn` (Float)
  - `disk_critical` (Float)

**sys-monitor**
Reports aggregated CPU utilization and memory usage.

Config:
  - `cpu_warn` (Float)
  - `cpu_critical` (Float)
  - `mem_warn` (Float)
  - `mem_critical` (Float)
