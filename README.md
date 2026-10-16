# AHP Matrix

Make better decisions with the [Analytic Hierarchy Process](https://en.wikipedia.org/wiki/Analytic_hierarchy_process):
compare your options two at a time, check that your judgments agree with each other, and see how robust the winner is.
AHP Matrix is a single binary that runs a local web app. Your projects stay on your machine as `.ahp.json` files.

![Comparing two criteria on the balance](docs/images/compare.jpg)

- **Compare in pairs on a balance.** Arrow keys tip it, Enter moves to the next pair, and you get useful results long before every pair is answered.
- **Rate or measure where pairs are overkill.** Each criterion can be compared in pairs, rated 1–10, or given measured values such as price ("lower is better").
- **Group criteria** into a hierarchy, so you only compare criteria that belong together.
- **Catch contradictions.** Every set of comparisons gets a consistency ratio, and the app points out the answers that disagree with the rest, with a one-click fix.
- **Several people.** Everyone judges separately, the results combine them (weighted), and the app shows where people disagree most. Send someone their own copy, then merge it back when they return it.
- **What if?** Drag a criterion's weight and watch the ranking change. Each criterion shows how far it can move before the winner changes.
- **Bring your spreadsheet.** Import workbooks made with the classic "Priority Matrix" Excel template, and export results to Excel or CSV.
- **Start from an example or a template**, including vendor selection with the template's 23 criteria, grouped.

![Results for a laptop decision](docs/images/results.jpg)

## Install

Download the archive for your system from [Releases](https://github.com/Xafloc/ahp-matrix/releases), unpack it, and run `ahp-matrix`.
It starts on http://127.0.0.1:7777 and opens your browser. On macOS, Homebrew works too:

```sh
brew install xafloc/tap/ahp-matrix
```

`ahp-matrix --help` lists the flags (`--port`, `--host`, `--data-dir`, `--no-open`).
Projects are stored in `~/Library/Application Support/ahp-matrix` (macOS),
`%AppData%\ahp-matrix` (Windows) or `~/.local/share/ahp-matrix` (Linux) unless `--data-dir` is given.
Each decision is one `.ahp.json` file; deleted ones move to a `trash` folder there.

### Build from source

Requires Go 1.24+ and Node 20.19+.

```sh
git clone https://github.com/Xafloc/ahp-matrix && cd ahp-matrix
make run    # builds the UI and the binary, then starts it
```

## Development

```sh
make test        # Go tests (race detector)
make lint        # gofmt + go vet
make web-check   # type-check and unit-test the UI
make e2e         # end-to-end browser tests against the real binary
make snapshot    # build release archives for every platform into dist/
```

Releases are cut by pushing a `v*` tag; `.github/workflows/release.yml` runs GoReleaser.

For live-reloading UI work, run the API in one terminal (`make build && bin/ahp-matrix --no-open`)
and `cd web && npm run dev` in another, then open http://localhost:5173.

## License

MIT
