# LogProxy

A lightweight log collection server written in Go, supporting daily log rotation and size-based backup.

## Features

- HTTP-based log collection via `POST /logs`
- Automatic daily log rotation
- Size-based log rotation with configurable max file size (default 64MB)
- Configurable backup count (default 10)
- Thread-safe concurrent log writing

## Usage

```bash
go run main.go
```

The server starts on `0.0.0.0:9554`.

## API

**POST /logs**

Send log lines as plain text, each line separated by `\n`.

```bash
curl -X POST http://localhost:9554/logs -d "log line 1\nlog line 2"
```

## Log Directory

Logs are stored in the `logs/` directory with the following naming convention:

- Current log: `YYYY-MM-DD.0.log`
- Backups: `YYYY-MM-DD.1.log`, `YYYY-MM-DD.2.log`, etc.

## License

MIT
