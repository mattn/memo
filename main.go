package main

import (
	"bufio"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
	"github.com/mattn/go-shellwords"
	"github.com/mattn/go-tty"
	"github.com/pkg/browser"
	"github.com/shurcooL/github_flavored_markdown"
	"github.com/shurcooL/github_flavored_markdown/gfmstyle"
	"github.com/urfave/cli"
)

const (
	column = 30

	// VERSION is a version of this app
	VERSION = "0.0.4"
)

const templateDirContent = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Memo Life For You</title>
</head>
<style>
li {list-style-type: none;}
</style>
<body>
<ul>{{range .}}
  <li><a href="/{{.Name}}">{{.Name}}</a><dd>{{.Body}}</dd></li>{{end}}
</ul>
</body>
</html>
`

const templateBodyContent = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>{{.Name}}</title>
  <link href="/assets/gfm/gfm.css" media="all" rel="stylesheet" type="text/css" />
</head>
<body>
{{.Body}}</body>
</html>
`

const templateMemoContent = `# {{.Title}}

`

type config struct {
	MemoDir   string `toml:"memodir"`
	Editor    string `toml:"editor"`
	Column    int    `toml:"column"`
	SelectCmd string `toml:"selectcmd"`
	GrepCmd   string `toml:"grepcmd"`
	AssetsDir string `toml:"assetsdir"`
}

type entry struct {
	Name string
	Body template.HTML
}

var commands = []cli.Command{
	{
		Name:    "new",
		Aliases: []string{"n"},
		Usage:   "create memo",
		Action:  cmdNew,
	},
	{
		Name:    "list",
		Aliases: []string{"l"},
		Usage:   "list memo",
		Action:  cmdList,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "fullpath",
				Usage: "show file path",
			},
		},
	},
	{
		Name:    "edit",
		Aliases: []string{"e"},
		Usage:   "edit memo",
		Action:  cmdEdit,
	},
	{
		Name:    "delete",
		Aliases: []string{"d"},
		Usage:   "delete memo",
		Action:  cmdDelete,
	},
	{
		Name:    "grep",
		Aliases: []string{"g"},
		Usage:   "grep memo",
		Action:  cmdGrep,
	},
	{
		Name:    "config",
		Aliases: []string{"c"},
		Usage:   "configure",
		Action:  cmdConfig,
	},
	{
		Name:    "serve",
		Aliases: []string{"s"},
		Usage:   "start http server",
		Action:  cmdServe,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "addr",
				Value: ":8080",
				Usage: "server address",
			},
		},
	},
}

func (cfg *config) load() error {
	var dir string
	if runtime.GOOS == "windows" {
		dir = os.Getenv("APPDATA")
		if dir == "" {
			dir = filepath.Join(os.Getenv("USERPROFILE"), "Application Data", "memo")
		}
		dir = filepath.Join(dir, "memo")
	} else {
		dir = filepath.Join(os.Getenv("HOME"), ".config", "memo")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create directory: %v", err)
	}
	file := filepath.Join(dir, "config.toml")

	_, err := os.Stat(file)
	if err == nil {
		_, err := toml.DecodeFile(file, cfg)
		if err != nil {
			return err
		}
		dir := os.Getenv("MEMODIR")
		if dir != "" {
			cfg.MemoDir = dir
		}
		return nil
	}

	if !os.IsNotExist(err) {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	dir = filepath.Join(dir, "_posts")
	os.MkdirAll(dir, 0700)
	cfg.MemoDir = filepath.ToSlash(dir)
	cfg.Editor = os.Getenv("EDITOR")
	if cfg.Editor == "" {
		cfg.Editor = "vim"
	}
	cfg.Column = 20
	cfg.SelectCmd = "peco"
	cfg.GrepCmd = "grep -nH ${PATTERN} ${FILES}"
	cfg.AssetsDir = "."

	dir = os.Getenv("MEMODIR")
	if dir != "" {
		cfg.MemoDir = dir
	}
	return toml.NewEncoder(f).Encode(cfg)
}

func msg(err error) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		return 1
	}
	return 0
}

func filterMarkdown(files []string) []string {
	var newfiles []string
	for _, file := range files {
		if strings.HasSuffix(file, ".md") {
			newfiles = append(newfiles, file)
		}
	}
	sort.Strings(newfiles)
	return newfiles
}

func ask(prompt string) (bool, error) {
	fmt.Print(prompt + ": ")
	t, err := tty.Open()
	if err != nil {
		return false, err
	}
	defer t.Close()
	var r rune
	for r == 0 {
		r, err = t.ReadRune()
		if err != nil {
			return false, err
		}
	}
	fmt.Println()
	return r == 'y' || r == 'Y', nil
}

func run() int {
	app := cli.NewApp()
	app.Name = "memo"
	app.Usage = "Memo Life For You"
	app.Version = VERSION
	app.Commands = commands
	return msg(app.Run(os.Args))
}

func firstline(name string) string {
	b, err := ioutil.ReadFile(name)
	if err != nil {
		return ""
	}
	body := string(b)
	if strings.HasPrefix(body, "---\n") {
		if pos := strings.Index(body[4:], "---\n"); pos > 0 {
			body = body[4+pos+4:]
		}
	}
	body = strings.SplitN(strings.TrimSpace(body), "\n", 2)[0]
	return strings.TrimLeft(body, "# ")
}

func cmdList(c *cli.Context) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}

	f, err := os.Open(cfg.MemoDir)
	if err != nil {
		return err
	}
	defer f.Close()
	files, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}
	files = filterMarkdown(files)
	istty := isatty.IsTerminal(os.Stdout.Fd())
	col := cfg.Column
	if col == 0 {
		col = column
	}
	pat := c.Args().First()
	fullpath := c.Bool("fullpath")
	for _, file := range files {
		if pat != "" && !strings.Contains(file, pat) {
			continue
		}
		if istty && !fullpath {
			title := runewidth.Truncate(firstline(filepath.Join(cfg.MemoDir, file)), 80-4-col, "...")
			file = runewidth.FillRight(runewidth.Truncate(file, col, "..."), col)
			fmt.Fprintf(color.Output, "%s : %s\n", color.GreenString(file), color.YellowString(title))
		} else {
			if fullpath {
				file = filepath.Join(cfg.MemoDir, file)
			}
			fmt.Println(file)
		}
	}
	return nil
}

func escape(name string) string {
	s := regexp.MustCompile(`[ <>:"/\\|?*%#]`).ReplaceAllString(name, "-")
	s = regexp.MustCompile(`--+`).ReplaceAllString(s, "-")
	return strings.Trim(strings.Replace(s, "--", "-", -1), "- ")
}

func (cfg *config) runcmd(command, pattern string, files ...string) error {
	var args []string
	for _, file := range files {
		args = append(args, shellquote(file))
	}
	cmdargs := strings.Join(args, " ")

	hasEnv := false
	command = os.Expand(command, func(s string) string {
		hasEnv = true
		switch s {
		case "FILES":
			return cmdargs
		case "PATTERN":
			return pattern
		case "DIR":
			return cfg.MemoDir
		}
		return ""
	})
	if !hasEnv {
		command += " " + cmdargs
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func cmdNew(c *cli.Context) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}

	var title string
	var file string
	if c.Args().Present() {
		title = c.Args().First()
	} else {
		fmt.Print("Title: ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return errors.New("canceld")
		}
		if scanner.Err() != nil {
			return scanner.Err()
		}
		title = scanner.Text()
	}
	file = time.Now().Format("2006-01-02-") + escape(title) + ".md"
	file = filepath.Join(cfg.MemoDir, file)
	t := template.Must(template.New("memo").Parse(templateMemoContent))
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	err = t.Execute(f, struct {
		Title string
	}{
		title,
	})
	f.Close()
	if err != nil {
		return err
	}
	return cfg.runcmd(cfg.Editor, "", file)
}

func cmdEdit(c *cli.Context) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}

	var files []string
	if c.Args().Present() {
		files = append(files, filepath.Join(cfg.MemoDir, c.Args().First()))
	} else {
		f, err := os.Open(cfg.MemoDir)
		if err != nil {
			return err
		}
		defer f.Close()
		files, err = f.Readdirnames(-1)
		if err != nil {
			return err
		}
		files = filterMarkdown(files)
		cmds, err := shellwords.Parse(cfg.SelectCmd)
		if len(cmds) == 0 || err != nil {
			return errors.New("selectcmd: invalid commands")
		}
		cmd := exec.Command(cmds[0], cmds[1:]...)
		cmd.Stdin = strings.NewReader(strings.Join(files, "\n"))
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: you need to install peco first: https://github.com/peco/peco", err)
		}
		files = strings.Split(strings.TrimSpace(string(b)), "\n")
		for i, file := range files {
			files[i] = filepath.Join(cfg.MemoDir, file)
		}
	}
	return cfg.runcmd(cfg.Editor, "", files...)
}

func cmdDelete(c *cli.Context) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}

	if !c.Args().Present() {
		return errors.New("pattern required")
	}
	f, err := os.Open(cfg.MemoDir)
	if err != nil {
		return err
	}
	defer f.Close()
	files, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}
	files = filterMarkdown(files)
	pat := c.Args().First()
	var args []string
	for _, file := range files {
		if pat != "" && !strings.Contains(file, pat) {
			continue
		}
		fmt.Println(file)
		args = append(args, filepath.Join(cfg.MemoDir, file))
	}
	if len(args) == 0 {
		color.Yellow("%s", "No matched entry")
		return nil
	}
	color.Red("%s", "Will delete those entry. Are you sure?")
	answer, err := ask("Are you sure? (y/N)")
	if answer == false || err != nil {
		return err
	}
	answer, err = ask("Really? (y/N)")
	if answer == false || err != nil {
		return err
	}
	for _, arg := range args {
		err = os.Remove(arg)
		if err != nil {
			return err
		}
		color.Yellow("Deleted: %v", arg)
	}
	return nil
}

func cmdGrep(c *cli.Context) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}

	if !c.Args().Present() {
		return errors.New("pattern required")
	}
	f, err := os.Open(cfg.MemoDir)
	if err != nil {
		return err
	}
	defer f.Close()
	files, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}
	files = filterMarkdown(files)
	var args []string
	for _, file := range files {
		args = append(args, filepath.Join(cfg.MemoDir, file))
	}
	return cfg.runcmd(cfg.GrepCmd, c.Args().First(), args...)
}

func cmdConfig(c *cli.Context) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}

	dir := os.Getenv("HOME")
	if dir == "" && runtime.GOOS == "windows" {
		dir = os.Getenv("APPDATA")
		if dir == "" {
			dir = filepath.Join(os.Getenv("USERPROFILE"), "Application Data", "memo")
		}
		dir = filepath.Join(dir, "memo")
	} else {
		dir = filepath.Join(dir, ".config", "memo")
	}
	file := filepath.Join(dir, "config.toml")
	return cfg.runcmd(cfg.Editor, "", file)
}

func cmdServe(c *cli.Context) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			f, err := os.Open(cfg.MemoDir)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer f.Close()
			files, err := f.Readdirnames(-1)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			files = filterMarkdown(files)
			var entries []entry
			for _, file := range files {
				entries = append(entries, entry{
					Name: file,
					Body: template.HTML(runewidth.Truncate(firstline(filepath.Join(cfg.MemoDir, file)), 80, "...")),
				})
			}
			w.Header().Set("content-type", "text/html")
			t := template.Must(template.New("dir").Parse(templateDirContent))
			err = t.Execute(w, entries)
			if err != nil {
				log.Println(err)
			}
		} else {
			p := filepath.Join(cfg.MemoDir, escape(req.URL.Path))
			b, err := ioutil.ReadFile(p)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			body := string(b)
			if strings.HasPrefix(body, "---\n") {
				if pos := strings.Index(body[4:], "---\n"); pos > 0 {
					body = body[4+pos+4:]
				}
			}
			body = string(github_flavored_markdown.Markdown([]byte(body)))
			t := template.Must(template.New("body").Parse(templateBodyContent))
			t.Execute(w, entry{
				Name: req.URL.Path,
				Body: template.HTML(body),
			})
		}
	})
	http.Handle("/assets/gfm/", http.StripPrefix("/assets/gfm", http.FileServer(gfmstyle.Assets)))
	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir(cfg.AssetsDir))))

	addr := c.String("addr")
	var url string
	if strings.HasPrefix(addr, ":") {
		url = "http://localhost" + addr
	} else {
		url = "http://" + addr
	}
	browser.OpenURL(url)
	return http.ListenAndServe(addr, nil)
}

func main() {
	os.Exit(run())
}
