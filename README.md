# govna

A gradual port of [governa](https://github.com/queone/governa) from Go to Rust. Not a drop-in replacement for, or dependency of, governa while the port is in progress.

## Commands

| Command | Status |
| --- | --- |
| `version`, `ver`, `v`, `--version` | implemented |
| `-h`, `--help`, `-?`, `help`, `h` | implemented |
| `apply` | implemented |
| `drift-scan` | implemented |
| `rm` | implemented |
| `deps` | not yet implemented |
| `render-canon` | implemented |

## Build

```
cargo build
```

## Test

```
cargo test
```
