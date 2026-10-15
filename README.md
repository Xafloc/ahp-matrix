# AHP Matrix

Make better decisions with the [Analytic Hierarchy Process](https://en.wikipedia.org/wiki/Analytic_hierarchy_process):
compare your options two at a time, check that your judgments agree with each other, and see how robust the winner is.
AHP Matrix is a single binary that runs a local web app. Your projects stay on your machine as `.ahp.json` files.

![Comparing two criteria on the balance](docs/images/compare.jpg)

- **Compare in pairs on a balance.** Arrow keys tip it, Enter moves to the next pair, and you get useful results long before every pair is answered.
- **Rate or measure where pairs are overkill.** Each criterion can be compared in pairs, rated 1–10, or given measured values such as price ("lower is better").
- **Group criteria** into a hierarchy, so you only compare criteria that belong together.
- **Catch contradictions.** Every set of comparisons gets a consistency ratio, and the app points out the answers that disagree with the rest, with a one-click fix.
- **Several people.** Everyone judges separately, the results combine them (weighted), and the app shows where people disagree most.
- **What if?** Drag a criterion's weight and watch the ranking change. Each criterion shows how far it can move before the winner changes.

![Results for a laptop decision](docs/images/results.jpg)

> **Status:** early development. Excel import/export, sample projects and release binaries are next.

## Run it

Requires Go 1.24+ and Node 20.19+ to build from source.

```sh
git clone https://github.com/Xafloc/ahp-matrix && cd ahp-matrix
make run    # builds the UI and the binary, starts on http://127.0.0.1:7777 and opens your browser
```

`bin/ahp-matrix --help` lists the flags (`--port`, `--host`, `--data-dir`, `--no-open`).
Projects are stored in `~/Library/Application Support/ahp-matrix` (macOS),
`%AppData%\ahp-matrix` (Windows) or `~/.local/share/ahp-matrix` (Linux) unless `--data-dir` is given.

## Development

```sh
make test        # Go tests (race detector)
make lint        # gofmt + go vet
make web-check   # type-check and unit-test the UI
make e2e         # end-to-end browser tests against the real binary
```

For live-reloading UI work, run the API in one terminal (`make build && bin/ahp-matrix --no-open`)
and `cd web && npm run dev` in another, then open http://localhost:5173.

## License

MIT
