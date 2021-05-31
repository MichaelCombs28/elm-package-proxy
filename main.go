package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/MichaelCombs28/elm-package-proxy/elmproxy"

	"github.com/elazarl/goproxy"
)

func main() {
	listen := flag.String("proxy-listen", "localhost:8080", "Proxy Host string")
	addr := flag.String("api-listen", "localhost:8081", "API server host string")
	logLevel := flag.String("log-level", "INFO", "Log Level PANIC, FATAL, INFO, DEBUG, TRACE")
	flag.Parse()

	// Signals
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Set logging
	log.SetFormatter(&log.JSONFormatter{})
	level, err := log.ParseLevel(*logLevel)
	orPanic(err)
	log.SetLevel(level)

	// Retrieve certs
	// TODO CU-wndcr3: Overwrite certificate store to only provide package.elm-lang.org certificate
	caCert, err := ioutil.ReadFile("./ca.crt")
	orPanic(err)
	caKey, err := ioutil.ReadFile("./ca.key")
	orPanic(err)
	orPanic(setCA(caCert, caKey))

	log.Info("Initializing...")
	err = elmproxy.Initialize()
	orPanic(err)

	// Initializing Sync worker
	mux := elmproxy.ProxyHandler()
	ctx, cancel := context.WithCancel(context.Background())
	go elmproxy.SyncWorker(ctx)

	// Proxy setup
	proxy := goproxy.NewProxyHttpServer()
	proxy.Logger = &NopLogger{}
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
		HandleConnect(goproxy.AlwaysMitm)
	proxy.OnRequest(goproxy.DstHostIs("package.elm-lang.org:443")).DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if resp := mux(r); resp != nil {
			return r, resp
		}

		//TODO remove
		if strings.Contains(r.URL.Path, "elm.json") || strings.Contains(r.URL.Path, "endpoint.json") || strings.Contains(r.URL.Path, "docs.json") && r.Method == "GET" {
			return r, nil
		}
		//return r, nil
		return r, goproxy.NewResponse(r, goproxy.ContentTypeText, 500, "")
	})
	proxy.OnRequest(goproxy.DstHostIs(*listen)).DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		return r, mux(r)
	})

	log.Printf("Starting Proxy Server on %s", *listen)
	go func() {
		if err := http.ListenAndServe(*listen, proxy); err != nil {
			if err != http.ErrServerClosed {
				log.Fatal("Server threw an error.", err)
			}
		}
	}()
	srv := &http.Server{
		Addr:    *addr,
		Handler: elmproxy.Router(),
	}
	log.Printf("Starting API Server on %s", *addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("API server unexpected close. ", err)
		}
	}()
	<-done
	cancel()
	ctx, can := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		can()
	}()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("API shutdown failed: %+v", err)
	}
}

func setCA(caCert, caKey []byte) error {
	goproxyCa, err := tls.X509KeyPair(caCert, caKey)
	if err != nil {
		return err
	}
	if goproxyCa.Leaf, err = x509.ParseCertificate(goproxyCa.Certificate[0]); err != nil {
		return err
	}
	goproxy.GoproxyCa = goproxyCa
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	return nil
}

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

// Used to silence internal logging used by goproxy
//
type NopLogger struct{}

func (_ *NopLogger) Printf(format string, v ...interface{}) {}
