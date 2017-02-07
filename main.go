package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
	"github.com/urfave/cli"
)

const column = 20

type config struct {
	MemoDir   string `toml:"memodir"`
	Editor    string `toml:"editor"`
	Column    int    `toml:"column"`
	SelectCmd string `toml:"selectcmd"`
	GrepCmd   string `toml:"GrepCmd"`
}

func loadConfig(cfg *config) error {
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
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file := filepath.Join(dir, "config.toml")

	_, err := os.Stat(file)
	if err == nil {
		_, err := toml.DecodeFile(file, cfg)
		return err
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
	cfg.Editor = "vim"
	cfg.Column = 20
	cfg.SelectCmd = "peco"
	cfg.GrepCmd = "grep"
	return toml.NewEncoder(f).Encode(cfg)
}

func msg(err error) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		return 1
	}
	return 0
}

func run() int {
	var cfg config
	err := loadConfig(&cfg)
	if err != nil {
		return msg(err)
	}

	app := cli.NewApp()
	app.Name = "memo"
	app.Usage = "Memo Life For You"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
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
		},
		{
			Name:    "edit",
			Aliases: []string{"e"},
			Usage:   "edit memo",
			Action:  cmdEdit,
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
	}
	app.Metadata = map[string]interface{}{
		"config": &cfg,
	}
	return msg(app.Run(os.Args))
}

func firstline(name string) string {
	f, err := os.Open(name)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return ""
	}
	if scanner.Err() != nil {
		return ""
	}
	return strings.TrimLeft(scanner.Text(), "# ")
}

func cmdList(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config)
	f, err := os.Open(cfg.MemoDir)
	if err != nil {
		return err
	}
	defer f.Close()
	files, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}
	istty := isatty.IsTerminal(os.Stdout.Fd())
	col := cfg.Column
	if col == 0 {
		col = column
	}
	for _, file := range files {
		if istty {
			title := runewidth.Truncate(firstline(filepath.Join(cfg.MemoDir, file)), 80-col, "...")
			file = runewidth.FillRight(runewidth.Truncate(file, col, "..."), col)
			fmt.Fprintf(color.Output, "%s : %s\n", color.GreenString(file), color.YellowString(title))
		} else {
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

func edit(editor string, files ...string) error {
	cmd := exec.Command(editor, files...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdNew(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config)

	var file string
	if c.Args().Present() {
		file = c.Args().First()
	} else {
		fmt.Print("Title: ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return errors.New("canceld")
		}
		if scanner.Err() != nil {
			return scanner.Err()
		}
		file = scanner.Text()
	}
	file = time.Now().Format("2006-01-02-") + escape(file) + ".md"
	file = filepath.Join(cfg.MemoDir, file)
	return edit(cfg.Editor, file)
}

func cmdEdit(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config)

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
		cmd := exec.Command(cfg.SelectCmd)
		cmd.Stdin = strings.NewReader(strings.Join(files, "\n"))
		b, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		files = strings.Split(strings.TrimSpace(string(b)), "\n")
		for i, file := range files {
			files[i] = filepath.Join(cfg.MemoDir, file)
		}
	}
	return edit(cfg.Editor, files...)
}

func cmdGrep(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config)

	if !c.Args().Present() {
		return nil
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
	var args []string
	args = append(args, c.Args().First())
	for _, file := range files {
		args = append(args, filepath.Join(cfg.MemoDir, file))
	}
	cmd := exec.Command(cfg.GrepCmd, args...)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func cmdConfig(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config)

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
	return edit(cfg.Editor, file)
}

func main() {
	os.Exit(run())
}
