# memo

Memo Life For You

![Memo Life For You](https://raw.githubusercontent.com/mattn/memo/master/screenshot.gif)

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
grepcmd = "grep -nH"              # grep command executable
assetsdir = "/path/to/assets"     # assets directory for serve command
```

editor, selectcmd, grepcmd can be used placeholder below.

|placeholder|replace to     |
|-----------|---------------|
|${FILES}   |target files   |
|${DIR}     |same as memodir|
|${PATTERN} |grep pattern   |

## FAQ

### Want to use [ag](https://github.com/ggreer/the_silver_searcher) for grepcmd

```
grepcmd = "ag ${PATTERN} ${DIR}"
```

### Want to use [jvgrep](https://github.com/mattn/jvgrep) for grepcmd

```
grepcmd = "jvgrep ${PATTERN} ${DIR}"
```

### Want to use [gof](https://github.com/mattn/gof) for selectcmd

```
selectcmd = "gof"
```

### Want to use [fzf](https://github.com/junegunn/fzf) for selectcmd

```
selectcmd = "fzf"
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a. mattn)
