package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
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
	tt "text/template"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
	"github.com/mattn/go-tty"
	"github.com/pkg/browser"
	"github.com/shurcooL/github_flavored_markdown"
	"github.com/shurcooL/github_flavored_markdown/gfmstyle"
	"github.com/urfave/cli/v2"
)

const (
	column = 30
	width  = 80

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
	MemoDir          string `toml:"memodir"`
	Editor           string `toml:"editor"`
	Column           int    `toml:"column"`
	Width            int    `toml:"width"`
	SelectCmd        string `toml:"selectcmd"`
	GrepCmd          string `toml:"grepcmd"`
	MemoTemplate     string `toml:"memotemplate"`
	AssetsDir        string `toml:"assetsdir"`
	PluginsDir       string `toml:"pluginsdir"`
	TemplateDirFile  string `toml:"templatedirfile"`
	TemplateBodyFile string `toml:"templatebodyfile"`
}

type entry struct {
	Name string
	Body template.HTML
}

var commands = []*cli.Command{
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
			&cli.BoolFlag{
				Name:  "fullpath",
				Usage: "show file path",
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "print the result using a Go template `string`",
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
		Name:    "cat",
		Aliases: []string{"v"},
		Usage:   "view memo",
		Action:  cmdCat,
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
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "cat",
				Usage: "cat the file",
			},
		},
	},
	{
		Name:    "serve",
		Aliases: []string{"s"},
		Usage:   "start http server",
		Action:  cmdServe,
		Flags: []cli.Flag{
			&cli.StringFlag{
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

	confDir := dir

	_, err := os.Stat(file)
	if err == nil {
		_, err := toml.DecodeFile(file, cfg)
		if err != nil {
			return err
		}
		cfg.MemoDir = expandPath(cfg.MemoDir)
		cfg.AssetsDir = expandPath(cfg.AssetsDir)
		if cfg.PluginsDir == "" {
			cfg.PluginsDir = filepath.Join(confDir, "plugins")
		}
		cfg.PluginsDir = expandPath(cfg.PluginsDir)

		if cfg.MemoTemplate == "" {
			cfg.MemoTemplate = filepath.Join(confDir, "template.txt")
		}
		cfg.MemoTemplate = expandPath(cfg.MemoTemplate)

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
	dir = filepath.Join(confDir, "plugins")
	os.MkdirAll(dir, 0700)
	cfg.PluginsDir = filepath.ToSlash(dir)

	dir = os.Getenv("MEMODIR")
	if dir != "" {
		cfg.MemoDir = dir
	}
	return toml.NewEncoder(f).Encode(cfg)
}

func expandPath(s string) string {
	if len(s) >= 2 && s[0] == '~' && os.IsPathSeparator(s[1]) {
		if runtime.GOOS == "windows" {
			s = filepath.Join(os.Getenv("USERPROFILE"), s[2:])
		} else {
			s = filepath.Join(os.Getenv("HOME"), s[2:])
		}
	}
	return os.Expand(s, os.Getenv)
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
	sort.Sort(sort.Reverse(sort.StringSlice(newfiles)))
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
	app.Action = appRun

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

	var tmpl *tt.Template
	if format := c.String("format"); format != "" {
		t, err := tt.New("format").Parse(format)
		if err != nil {
			return err
		}
		tmpl = t
	}

	fullpath := c.Bool("fullpath")
	for _, file := range files {
		if pat != "" && !strings.Contains(file, pat) {
			continue
		}
		if tmpl != nil {
			var b bytes.Buffer
			err := tmpl.Execute(&b, map[string]interface{}{
				"File":     file,
				"Title":    firstline(filepath.Join(cfg.MemoDir, file)),
				"Fullpath": filepath.Join(cfg.MemoDir, file),
			})
			if err != nil {
				return err
			}
			fmt.Println(b.String())
		} else if istty && !fullpath {
			wi := cfg.Width
			if wi == 0 {
				wi = width
			}
			title := runewidth.Truncate(firstline(filepath.Join(cfg.MemoDir, file)), wi-4-col, "...")
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

func (cfg *config) runfilter(command string, r io.Reader, w io.Writer) error {
	command = os.Expand(command, func(s string) string {
		switch s {
		case "DIR":
			return cfg.MemoDir
		}
		return os.Getenv(s)
	})

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Stderr = os.Stderr
	cmd.Stdout = w
	cmd.Stdin = r
	return cmd.Run()
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
		return os.Getenv(s)
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
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func copyFromStdin(filename string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, os.Stdin)
	return err
}

func cmdNew(c *cli.Context) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}

	var title string
	var file string
	now := time.Now()
	if c.Args().Present() {
		title = c.Args().First()
		file = now.Format("2006-01-02-") + escape(title) + ".md"
	} else {
		fmt.Print("Title: ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return errors.New("canceled")
		}
		if scanner.Err() != nil {
			return scanner.Err()
		}
		title = scanner.Text()
		if title == "" {
			title = now.Format("2006-01-02")
			file = title + ".md"

		} else {
			file = now.Format("2006-01-02-") + escape(title) + ".md"
		}
	}
	file = filepath.Join(cfg.MemoDir, file)
	if fileExists(file) {
		if !isatty.IsTerminal(os.Stdin.Fd()) {
			return copyFromStdin(file)
		}
		return cfg.runcmd(cfg.Editor, "", file)
	}

	tmplString := templateMemoContent

	if fileExists(cfg.MemoTemplate) {
		b, err := ioutil.ReadFile(cfg.MemoTemplate)
		if err != nil {
			return err
		}
		tmplString = filterTmpl(string(b))
	}
	t := template.Must(template.New("memo").Parse(tmplString))

	f, err := os.Create(file)
	if err != nil {
		return err
	}

	err = t.Execute(f, struct {
		Title, Date, Tags, Categories string
	}{
		title, now.Format("2006-01-02 15:04"), "", "",
	})
	f.Close()
	if err != nil {
		return err
	}

	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return copyFromStdin(file)
	}
	return cfg.runcmd(cfg.Editor, "", file)
}

var filterReg = regexp.MustCompile(`{{_(.+?)_}}`)

func filterTmpl(tmpl string) string {
	return filterReg.ReplaceAllStringFunc(tmpl, func(substr string) string {
		m := filterReg.FindStringSubmatch(substr)
		return fmt.Sprintf("{{.%s}}", strings.Title(m[1]))
	})
}

func (cfg *config) filterFiles() ([]string, error) {
	var files []string
	f, err := os.Open(cfg.MemoDir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	files, err = f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	files = filterMarkdown(files)
	var buf bytes.Buffer
	err = cfg.runfilter(cfg.SelectCmd, strings.NewReader(strings.Join(files, "\n")), &buf)
	if err != nil {
		// TODO:
		// Some select tools return non-zero, and some return zero.
		// This part can't handle whether the command execute failure or
		// the select command exit non-zero.
		//return fmt.Errorf("%v: you need to install peco first: https://github.com/peco/peco", err)
		return nil, err
	}
	if buf.Len() == 0 {
		return nil, errors.New("No files selected")
	}
	files = strings.Split(strings.TrimSpace(buf.String()), "\n")
	for i, file := range files {
		files[i] = filepath.Join(cfg.MemoDir, file)
	}
	return files, nil
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
		files, err = cfg.filterFiles()
		if err != nil {
			return err
		}
	}
	return cfg.runcmd(cfg.Editor, "", files...)
}

func catFile(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	return scanner.Err()
}

func cmdCat(c *cli.Context) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}

	var files []string
	if c.Args().Present() {
		files = append(files, filepath.Join(cfg.MemoDir, c.Args().First()))
	} else {
		files, err = cfg.filterFiles()
		if err != nil {
			return err
		}
	}

	for i, file := range files {
		if i > 0 {
			// Print new page
			fmt.Println("\x12")
		}
		err = catFile(file)
		if err != nil {
			return err
		}
	}
	return nil
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
	if err != nil || len(files) == 0 {
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
	if runtime.GOOS == "windows" {
		dir = os.Getenv("APPDATA")
		if dir == "" {
			dir = filepath.Join(os.Getenv("USERPROFILE"), "Application Data", "memo")
		}
		dir = filepath.Join(dir, "memo")
	} else {
		dir = filepath.Join(dir, ".config", "memo")
	}
	file := filepath.Join(dir, "config.toml")
	if c.Bool("cat") {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(os.Stdout, f)
		return err
	}

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
			cfg.TemplateDirFile = expandPath(cfg.TemplateDirFile)
			var t *template.Template
			if cfg.TemplateDirFile == "" {
				t = template.Must(template.New("dir").Parse(templateDirContent))
			} else {
				t, err = template.ParseFiles(cfg.TemplateDirFile)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
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
			cfg.TemplateBodyFile = expandPath(cfg.TemplateBodyFile)
			var t *template.Template
			if cfg.TemplateBodyFile == "" {
				t = template.Must(template.New("body").Parse(templateBodyContent))
			} else {
				t, err = template.ParseFiles(cfg.TemplateBodyFile)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
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

func listPlugins(fn func(string)) error {
	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}
	dir, err := os.Open(cfg.PluginsDir)
	if err != nil {
		return err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	sort.Strings(names)

	fmt.Println("\nSUB COMMANDS:")

	if runtime.GOOS != "windows" {
		for _, name := range names {
			p := filepath.Join(cfg.PluginsDir, name)
			fi, err := os.Stat(p)
			if err != nil {
				continue
			}
			if m := fi.Mode(); !m.IsDir() && m&0111 != 0 {
				fn(filepath.Join(cfg.PluginsDir, fi.Name()))
			}
		}
	} else {
		pathext := strings.Split(strings.ToLower(os.Getenv("PATHEXT")), ";")
		for _, name := range names {
			for _, p := range pathext {
				if strings.ToLower(filepath.Ext(name)) == p {
					fn(filepath.Join(cfg.PluginsDir, name))
					break
				}
			}
		}
	}
	return nil
}

func appRun(c *cli.Context) error {
	args := c.Args()
	if !args.Present() {
		cli.ShowAppHelp(c)
		listPlugins(func(s string) {
			b, err := exec.Command(s, "-usage").CombinedOutput()
			if err != nil {
				return
			}
			s = filepath.Base(s)
			if runtime.GOOS == "windows" {
				s = s[:len(s)-len(filepath.Ext(s))]
			}
			fmt.Println("     " + s)
			lines := strings.Split(string(b), "\n")
			for _, line := range lines {
				fmt.Println("       " + line)
			}
		})
		return nil
	}

	var cfg config
	err := cfg.load()
	if err != nil {
		return err
	}
	xcmdpath := filepath.Join(cfg.PluginsDir, args.First())
	_, err = exec.LookPath(xcmdpath)
	if err != nil {
		return fmt.Errorf("'%s' is not a memo command. see 'memo help'", args.First())
	}

	// run external command as a memo subcommand.
	xargs := args.Tail()
	cmd := exec.Command(xcmdpath, xargs...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("MEMODIR=%s", cfg.MemoDir))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func main() {
	os.Exit(run())
}
