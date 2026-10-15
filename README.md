# AHP Matrix

Make better decisions with the [Analytic Hierarchy Process](https://en.wikipedia.org/wiki/Analytic_hierarchy_process):
compare options two at a time, check that your judgments are consistent, and see how robust the winner is.
AHP Matrix is a single binary that runs a local web app; your projects stay on your machine as `.ahp.json` files.

> **Status:** early development. The engine and API work; the web UI is coming next.

## Development

Requires Go 1.24 or newer.

```sh
make test   # run all tests with the race detector
make lint   # gofmt + go vet
make run    # build and start on http://127.0.0.1:7777
```

`ahp-matrix --help` lists the flags (`--port`, `--host`, `--data-dir`, `--no-open`).
Projects are stored in `~/Library/Application Support/ahp-matrix` (macOS),
`%AppData%\ahp-matrix` (Windows) or `~/.local/share/ahp-matrix` (Linux) unless `--data-dir` is given.

## License

MIT
