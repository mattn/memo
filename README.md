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
   0.0.4

COMMANDS:
     new, n     create memo
     list, l    list memo
     edit, e    edit memo
     delete, d  delete memo
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
pluginsdir = "path/to/plugins"    # plugins directory for plugin commands. default '~/.config/memo/plugins'.
```

memodir and assetsdir can be used `~/` prefix or `$HOME` or OS specific environment variables. editor, selectcmd and grepcmd can be used placeholder below.

|placeholder|replace to     |
|-----------|---------------|
|${FILES}   |target files   |
|${DIR}     |same as memodir|
|${PATTERN} |grep pattern   |

## Supported GrepCmd


|Command                                            |Configuration                       |
|---------------------------------------------------|------------------------------------|
|GNU Grep                                           |grepcmd = "grep -nH" #default       |
|[ag](https://github.com/ggreer/the_silver_searcher)|grepcmd = "ag ${PATTERN} ${DIR}"    |
|[jvgrep](https://github.com/mattn/jvgrep)          |grepcmd = "jvgrep ${PATTERN} ${DIR}"|

## Supported SelectCmd

|Command                               |Configuration    |
|--------------------------------------|-----------------|
|[gof](https://github.com/mattn/gof)   |selectcmd = "gof"|
|[cho](https://github.com/mattn/cho)   |selectcmd = "cho"|
|[fzf](https://github.com/junegunn/fzf)|selectcmd = "fzf"|

## Extend With Plugin Commands

You can extend memo with custom commands. 
Place an executable file in your `pluginsdir`, memo can use it as a subcommand.
For example, If you place `foo` file in your `pluginsdir`, you can run it by `memo foo`.

Plugin MUST handle `-usage` option to show briefly.

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a. mattn)
