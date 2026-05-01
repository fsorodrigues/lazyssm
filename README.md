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

Default configuration file path:

- Linux/macOS: `~/.config/lazyssm/config.yaml`
- Windows: `%APPDATA%\\lazyssm\\config.yaml`

Configuration uses a `services` list with the following fields per entry:

- `name`: Unique service identifier
- `description`: Documentation-facing service description
- `target`: AWS SSM target (e.g. EC2 instance ID)
- `ports`: Single port mapping (not a list) with `port` (remote service port)
and `localPort` (local forwarded port)
- `profile`: AWS profile to authenticate with
- `region`: AWS region for the service

Path flags (`-file`, `-log-file`, `-process-log-dir`) accept absolute paths and
home-relative paths in these forms: `~`, `~/...`, and `~\\...`.

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
| `-file`            | Linux/macOS: `~/.config/lazyssm/config.yaml`<br>Windows: `%APPDATA%\\lazyssm\\config.yaml` | Path to configuration file                               |
| `-debug`           | `false`                               | Enable debug logging                                     |
| `-log-file`        | Linux/macOS: `~/.local/state/lazyssm/lazyssm.log`<br>Windows: `%LOCALAPPDATA%\\lazyssm\\lazyssm.log` | Path to application log file                             |
| `-process-log-dir` | Linux/macOS: `~/.local/state/lazyssm/process`<br>Windows: `%LOCALAPPDATA%\\lazyssm\\process` | Directory for per-process logs                           |
| `-auth-command`    | `aws-mfa`                             | Command to run before starting AWS SSM sessions          |
| `-skip-auth`       | `false`                               | Skip auth preflight before starting AWS SSM sessions     |
| `-simulate`        | `false`                               | Use simulated services instead of AWS SSM sessions       |

By default, services run an auth preflight command (`aws-mfa`) and then start
`aws ssm start-session` using the `AWS-StartPortForwardingSession` document.
Use `-skip-auth` to bypass preflight. The `-simulate` flag switches to a local
script (`scripts/simulate_service.sh` on Linux/macOS,
`scripts/simulate_service.ps1` on Windows) for development and testing and
bypasses auth preflight.

Platform note: Linux/macOS can run the auth command inside an interactive PTY
modal. On unsupported platforms, lazyssm falls back to running the auth command
without the in-app PTY modal.

---

## Manage services

The interface has two panels: available services and running services.

Start a service from the available panel to add it to the running panel. Each
running service corresponds to an `aws ssm start-session` port forwarding
session (or a simulated process when using `-simulate`).

Stopping a running service (`ctrl+d` then `enter`) and quitting the app (`q` or
`ctrl+c`) both trigger managed shutdown that targets the full process tree:

- Linux/macOS: sends `SIGINT` to the process group, then escalates to `SIGKILL`
  if needed.
- Windows: runs `taskkill /PID <pid> /T`, then escalates to
  `taskkill /PID <pid> /T /F` if needed.

---

## Review bindings

| Keybinding       | Context                | Action                              |
|------------------|------------------------|-------------------------------------|
| `enter`          | Available services     | Start selected service              |
| `tab`            | Global                 | Switch between panels               |
| `ctrl+d`         | Running services       | Prompt delete for selected service  |
| `enter`          | Running services (delete prompt) | Confirm delete |
| `escape`         | Running services (delete prompt) | Cancel delete |
| `enter`          | Auth modal             | Submit input to auth command        |
| `ctrl+c`         | Auth modal             | Cancel auth command                 |
| `q` / `ctrl+c`   | Global                 | Quit and clean up all processes     |
| `ctrl+z`         | Global                 | Suspend the application             |

While the auth modal is open, typed input is forwarded directly to the auth
command.

---

## Debug

Application logs write by default to:

- Linux/macOS: `~/.local/state/lazyssm/lazyssm.log`
- Windows: `%LOCALAPPDATA%\\lazyssm\\lazyssm.log`

Per-process logs are stored by default in:

- Linux/macOS: `~/.local/state/lazyssm/process`
- Windows: `%LOCALAPPDATA%\\lazyssm\\process`

Each process log file is named `process-<name>.log`.

Enable debug logging with the `-debug` CLI flag to capture additional runtime details.
