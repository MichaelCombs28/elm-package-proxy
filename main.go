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
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/MichaelCombs28/elm-package-proxy/elmproxy"

	"github.com/elazarl/goproxy"
)

func main() {
	configFilePath := flag.String("config", "./config.yml", "Config file")
	flag.Parse()

	viper.SetDefault("services.proxy", "localhost:8080")
	viper.SetDefault("services.api", "localhost:8081")
	viper.SetDefault("global.logLevel", "INFO")
	viper.SetDefault("services.sync.interval", 600)
	viper.SetDefault("services.database.file", "db.sqlite3")
	viper.SetConfigFile(*configFilePath)
	viper.SetConfigType("yaml")

	orPanic(viper.ReadInConfig())

	proxyAddr := viper.GetString("services.proxy")
	apiAddr := viper.GetString("services.api")
	logLevel := viper.GetString("global.logLevel")

	// Signals
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Set logging
	log.SetFormatter(&log.JSONFormatter{})
	level, err := log.ParseLevel(logLevel)
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
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		log.Debugf("%s - %s%s", r.Method, r.URL.Host, r.URL.Path)
		return r, nil
	})
	proxy.OnRequest(goproxy.DstHostIs("package.elm-lang.org:443")).DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if resp := mux(r); resp != nil {
			return r, resp
		}

		return r, nil
		//return r, goproxy.NewResponse(r, goproxy.ContentTypeText, 500, "")
	})
	proxy.OnRequest(goproxy.DstHostIs("api.github.com:443")).DoFunc(addGithubToken)
	proxy.OnRequest(goproxy.DstHostIs("github.com:443")).DoFunc(addGithubToken)
	/*
		proxy.OnResponse(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			log.Debug(resp.Request.MultipartForm)
			//b, _ := httputil.DumpResponse(resp, true)
			return resp
		})
	*/

	log.Printf("Starting Proxy Server on %s", proxyAddr)
	go func() {
		if err := http.ListenAndServe(proxyAddr, proxy); err != nil {
			if err != http.ErrServerClosed {
				log.Fatal("Server threw an error.", err)
			}
		}
	}()
	srv := &http.Server{
		Addr:    apiAddr,
		Handler: elmproxy.Router(),
	}
	log.Printf("Starting API Server on %s", apiAddr)
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

func addGithubToken(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	log.Debugf("%s - %s%s", r.Method, r.URL.Host, r.URL.Path)
	if token := viper.GetString("credentials.github"); token != "" {
		log.Debug("Appending header to github request")
		r.Header.Add("Authorization", "token "+token)
	}
	return r, nil
}
