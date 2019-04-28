package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	flagExec     = flag.String("exec", "mmdc", "mermaid.cli executable (and extra args).")
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
	// Check if file exists, if not, run `germaid-cli`
	if _, err := os.Stat(path.Join(h.root, r.URL.Path)); !os.IsNotExist(err) {
		h.chain.ServeHTTP(w, r)
		return
	}

	ext := path.Ext(r.URL.Path)
	if ext != ".png" && ext != ".svg" && ext != ".pdf" {
		http.NotFound(w, r)
		return
	}

	basepath := path.Join(h.root, strings.TrimSuffix(r.URL.Path, ext))
	markdown := basepath + ".md"
	graph := basepath + ext

	if _, err := os.Stat(markdown); err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else if os.IsPermission(err) {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}

	args := append(mermaidArgs, "-i", markdown, "-o", graph)
	cmd := exec.Command(mermaidExec, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.chain.ServeHTTP(w, r)
}

func main() {
	flag.Parse()

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

	addr := fmt.Sprintf(":%d", *flagPort)

	logrus.Infof("Start mermaid generator at %s on HTTP %s%s", *flagFileRoot, addr, *flagHTTPRoot)
	http.Handle(*flagHTTPRoot, http.StripPrefix(*flagHTTPRoot, mermaidServer(*flagFileRoot)))
	log.Fatal(http.ListenAndServe(addr, nil))
}
