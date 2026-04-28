# lazyssm tui

A personal tools to solve the hell of managing a billion different AWS SSM
connection variants. This TUI helps me rationally start port forwarding
sessions via a simple yaml config, which maps services to targets, ports, and
AWS profiles.

---

## Install

Build from source using the Go toolchain:

```bash
go build -o lazyssm .
```

Move the resulting binary to a directory in your PATH to run it from anywhere.

You can also use `go install` if you know what you're doing.

---

## Configure

Default configuration file path is `~/.config/lazyssm/config.yaml`.

Configuration uses a `services` list with the following fields per entry:

- `name`: Unique service identifier
- `description`: Documentation-facing service description
- `target`: AWS SSM target (e.g. EC2 instance ID)
- `ports`: Single port mapping (not a list) with `port` (remote service port)
and `localPort` (local forwarded port)
- `profile`: AWS profile to authenticate with
- `region`: AWS region for the service

Example configuration:

```yaml
services:
  - name: rds-proxy
    description: Forward RDS proxy port to local machine
    target: i-0abc123def456
    ports:
      port: 5432
      localPort: 5433
    profile: prod-admin
    region: us-east-1

  - name: redis-cache
    description: Redis cache forwarding
    target: i-0def789abc012
    ports:
      port: 6379
      localPort: 6380
    profile: staging
    region: eu-west-1
```

---

## Use

Launch the application by running `lazyssm` with optional CLI flags:

| Flag               | Default                               | Description                                              |
|--------------------|---------------------------------------|----------------------------------------------------------|
| `-file`            | `~/.config/lazyssm/config.yaml`       | Path to configuration file                               |
| `-debug`           | `false`                               | Enable debug logging                                     |
| `-log-file`        | `~/.local/state/lazyssm/lazyssm.log`  | Path to application log file                             |
| `-process-log-dir` | `~/.local/state/lazyssm/process`      | Directory for per-process logs                           |
| `-simulate`        | `false`                               | Use simulated services instead of AWS SSM sessions       |

By default, services are started via `aws ssm start-session` using the
`AWS-StartPortForwardingSession` document. The `-simulate` flag switches to a
local script (`scripts/simulate_service.sh`) for development and testing.

---

## Manage services

The interface has two panels: available services and running services.

Start a service from the available panel to add it to the running panel. Each
running service corresponds to an `aws ssm start-session` port forwarding
session (or a simulated process when using `-simulate`).

---

## Review bindings

| Keybinding       | Context                | Action                              |
|------------------|------------------------|-------------------------------------|
| `enter`          | Available services     | Start selected service              |
| `tab`            | Global                 | Switch between panels               |
| `ctrl+d`         | Running services       | Prompt delete for selected service  |
| `enter`          | Running services (delete prompt) | Confirm delete |
| `escape`         | Running services (delete prompt) | Cancel delete |
| `q` / `ctrl+c`   | Global                 | Quit and clean up all processes     |
| `ctrl+z`         | Global                 | Suspend the application             |

---

## Debug

Application logs write to `~/.local/state/lazyssm/lazyssm.log` by default.

Per-process logs are stored in `~/.local/state/lazyssm/process` as `process-<name>.log`.

Enable debug logging with the `-debug` CLI flag to capture additional runtime details.
