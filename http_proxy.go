package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HTTPProxy http反向代理
var HTTPProxy httpProxy
var errNotFound = errors.New("not found")

type httpProxy struct {
	server       *http.Server
	reverseProxy *httputil.ReverseProxy
	mainHandler  http.Handler
}

func (*httpProxy) init(handler http.Handler) {
	HTTPProxy.reverseProxy = &httputil.ReverseProxy{
		Director: HTTPProxy.director,
	}

	HTTPProxy.server = &http.Server{
		Addr:    listen,
		Handler: http.HandlerFunc(HTTPProxy.serveHTTP),
	}

	HTTPProxy.mainHandler = handler
}

func (*httpProxy) listenAndServe() error {
	return HTTPProxy.server.ListenAndServe()
}

func (*httpProxy) serveHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	if !strings.HasPrefix(urlPath, "/amiba.io") {
		HTTPProxy.reverseProxy.ServeHTTP(w, r)
		return
	}

	pkgs := strings.Split(urlPath, "/")
	pkg := ""
	version := "latest"
	for i, l := 1, len(pkgs); i < l; i++ {
		if pkgs[i] == "@latest" {
			break
		}

		if pkgs[i] == "@v" {
			if (i + 1) < l {
				if pkgs[i+1] != "list" {
					version = strings.TrimSuffix(pkgs[i+1], ".info")
					version = strings.TrimSuffix(version, ".mod")
					version = strings.TrimSuffix(version, ".zip")
				}
			}
			break
		}

		if pkg != "" {
			pkg += "/"
		}

		pkg += pkgs[i]
	}

	err := HTTPProxy.goModDownload(pkg, version)
	if err != nil && err != errNotFound {
		ReturnServerError(w, err)
		return
	}

	// 总是检查是否有缓存
	state := "finding"
	if _, e := os.Stat(filepath.Join(cacheDir, r.URL.Path)); e == nil {
		if err == errNotFound {
			state = "exists"
		}
		err = nil
	} else {
		err = errNotFound
	}

	if err == errNotFound {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(""))
		return
	}

	fmt.Fprintf(os.Stdout, "goproxy: %s %s.\n", pkg, state)
	HTTPProxy.mainHandler.ServeHTTP(w, r)
}

func (*httpProxy) director(r *http.Request) {
	r.URL.Scheme = "https"
	r.URL.Host = "goproxy.io"
}

func (*httpProxy) goModDownload(pkg, version string) (err error) {
	cmd := exec.Command("go", "mod", "download", pkg+"@"+version)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return
	}

	if err = cmd.Start(); err != nil {
		return
	}

	bytesErr, err := ioutil.ReadAll(stderr)
	if err != nil {
		return
	}

	_, err = ioutil.ReadAll(stdout)
	if err != nil {
		return
	}

	if err = cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "goModDownload: download %s stderr:\n%s", pkg, string(bytesErr))
		return
	}

	out := fmt.Sprintf("%s", bytesErr)
	err = errNotFound

	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) != 4 {
			continue
		}

		if f[1] == "finding" && f[2] == pkg {
			err = nil
			return
		}
	}

	return
}
