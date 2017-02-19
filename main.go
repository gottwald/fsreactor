package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"

	"gopkg.in/yaml.v2"
)

// Config holds the configfile data structure
type Config struct {
	Watchers []FSWatcherConfig `yaml:"watchers"`
}

// FSWatcherConfig holds the config data of a single watcher
// with all it's actions
type FSWatcherConfig struct {
	Path    string   `yaml:"path"`
	Actions []string `yaml:"actions"`
}

var configpath string

func init() {
	flag.StringVar(&configpath, "c", "", "path to the config file")
}

func readConfig() *Config {
	f, err := os.Open(configpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not open config file, got: %s\n", err)
		os.Exit(1)
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read config file at %s, got: %s\n", configpath, err)
		os.Exit(1)
	}
	conf := Config{}
	err = yaml.Unmarshal(b, &conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not parse config file at %s, got: %s\n", configpath, err)
		os.Exit(1)
	}
	return &conf
}

func walkWatchersActions(confWatchers []FSWatcherConfig, event fsnotify.Event) {
	for _, wconfig := range confWatchers {
		if strings.HasPrefix(event.Name, wconfig.Path) {
			// watcher matches, run all it's actions
			for _, action := range wconfig.Actions {
				if out, err := exec.Command(action).CombinedOutput(); err != nil {
					fmt.Fprintf(os.Stderr, "error running the action '%s', got: %s\n", action, err)
				} else {
					fmt.Fprintf(os.Stdout, "ran action %s, got: %s\n", action, out)
				}
			}
		}
	}
}

func main() {
	flag.Parse()
	if len(configpath) < 1 {
		fmt.Fprintln(os.Stderr, "missing config")
		os.Exit(1)
	}

	// Read yaml config
	conf := readConfig()

	// register sig channel to receive TERM signals
	// so we can close watchers etc
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGTERM)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not create a filesystem watcher, got: %s\n", err)
		os.Exit(1)
	}
	defer watcher.Close()

	done := make(chan struct{})
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if (event.Op&fsnotify.Create == fsnotify.Create) || (event.Op&fsnotify.Write == fsnotify.Write) {
					fmt.Fprintf(os.Stdout, "got a change event for %s\n", event.Name)
					walkWatchersActions(conf.Watchers, event)
				}
			case err := <-watcher.Errors:
				fmt.Fprintf(os.Stderr, "recieved a file system watcher error: %s\n", err)
			case <-sig:
				done <- struct{}{}
				return
			}
		}
	}()

	for _, wconfig := range conf.Watchers {
		err := watcher.Add(wconfig.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not watch directory %s, got: %s\n", wconfig.Path, err)
		} else {
			fmt.Fprintf(os.Stdout, "added filesystem watcher for %s\n", wconfig.Path)
		}
	}

	<-done
}
