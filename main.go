package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	flagExec     = flag.String("exec", "mmdc", "mermaid.cli executable (and extra args).")
	flagWidth    = flag.Int("width", 980, "Default graph width.")
	flagHeight   = flag.Int("height", 1080, "Default graph height.")
	flagPort     = flag.Int("port", 8100, "HTTP server port.")
	flagHTTPRoot = flag.String("httpRoot", "/mermaid/", "HTTP serving root.")
	flagFileRoot = flag.String("fileRoot", "./", "Root path of serving files.")
)

var (
	mermaidExec string
	mermaidArgs []string
)

type mermaidHandler struct {
	root  string
	chain http.Handler
}

func mermaidServer(root string) http.Handler {
	return &mermaidHandler{
		root:  root,
		chain: http.FileServer(http.Dir(root)),
	}
}

func (h *mermaidHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	base, width, height, ext := parseGraphURL(r.URL.Path)
	if ext != ".png" && ext != ".svg" && ext != ".pdf" {
		h.chain.ServeHTTP(w, r)
		return
	}

	// calculate file path and graph width height
	basepath := path.Join(h.root, base)
	mermaid := basepath + ".mmd"
	var graph string
	if width == "" {
		graph = basepath + ext
		width, height = strconv.Itoa(*flagWidth), strconv.Itoa(*flagHeight)
	} else {
		graph = basepath + "." + width + "x" + height + ext
	}

	if mdStat, _ := os.Stat(mermaid); mdStat != nil {
		graphStat, err := os.Stat(graph)
		if os.IsNotExist(err) || graphStat != nil && graphStat.ModTime().Before(mdStat.ModTime()) {
			if err := makeGraph(graph, mermaid, width, height); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	h.chain.ServeHTTP(w, r)
	return
}

func parseGraphURL(url string) (base, width, height, ext string) {
	ext = path.Ext(url)
	base = strings.TrimSuffix(url, ext)
	if wh := path.Ext(base); wh != "" {
		pair := strings.SplitN(wh[1:], "x", 2)
		if len(pair) == 2 {
			_, err1 := strconv.Atoi(pair[0])
			_, err2 := strconv.Atoi(pair[1])
			if err1 == nil && err2 == nil {
				width, height = pair[0], pair[1]
				base = strings.TrimSuffix(base, wh)
			}
		}
	}
	return
}

func makeGraph(dest, src, width, height string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	args := append(mermaidArgs, "-w", width, "-H", height, "-i", src, "-o", dest)
	cmd := exec.CommandContext(ctx, mermaidExec, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	err := cmd.Run()
	return err
}

func parseCmd() {
	cmd := strings.Split(strings.TrimSpace(*flagExec), " ")
	if len(cmd) == 0 {
		logrus.Fatal("-exec cannot be empty.")
	}
	mermaidExec = cmd[0]
	mermaidArgs = make([]string, 0, len(cmd)+10)
	for _, arg := range cmd[1:] {
		arg = strings.TrimSpace(arg)
		if arg != "" {
			mermaidArgs = append(mermaidArgs, arg)
		}
	}
}

func main() {
	flag.Parse()
	parseCmd()
	addr := fmt.Sprintf(":%d", *flagPort)

	logrus.Infof("Start mermaid generator at %s on HTTP %s%s", *flagFileRoot, addr, *flagHTTPRoot)
	http.Handle(*flagHTTPRoot, http.StripPrefix(*flagHTTPRoot, mermaidServer(*flagFileRoot)))
	log.Fatal(http.ListenAndServe(addr, nil))
}
