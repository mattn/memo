# memo

Memo Life For You

## Usage

```
NAME:
   memo - Memo Life For You

USAGE:
   memo [global options] command [command options] [arguments...]

VERSION:
   0.0.1

COMMANDS:
     new, n     create memo
     list, l    list memo
     edit, e    edit memo
     grep, g    grep memo
     config, c  configure
     serve, s   start http server
     help, h    Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```

## Installation

```
$ go get github.com/mattn/memo
```

Let's start create memo file.

```
$ memo new
Title:
```

Input title for the memo, then you see the text editor launched. After saving markdown, list entries with `memo list`.

```
$ memo list
2017-02-07-memo-command.md   : Installed memo command
```

And grep

```
$ memo grep command
2017-02-07-memo-command.md:1:# Installed memo command
```

## Configuration

run `memo config`.

```toml
memodir = "/path/to/you/memo/dir" # specify memo directory
editor = "vim"                    # your favorite text editor
column = 30                       # column size for list command
selectcmd = "peco"                # selector command for edit command
grepcmd = "grep -n"               # grep command executable
assetsdir = "/path/to/assets"     # assets directory for serve command
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a. mattn)
