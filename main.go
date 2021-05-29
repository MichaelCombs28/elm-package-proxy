package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/MichaelCombs28/elm-package-proxy/elmproxy"

	"github.com/elazarl/goproxy"
)

var dataDir = "./data"

type NopLogger struct{}

func (_ *NopLogger) Printf(format string, v ...interface{}) {}

func main() {
	// Set logging
	log.SetFormatter(&log.JSONFormatter{})
	logLevel := flag.String("log-level", "INFO", "Log Level PANIC, FATAL, INFO, DEBUG, TRACE")
	flag.Parse()
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

	mux := elmproxy.Initialize()
	log.Info("Initializing...")
	_, err = elmproxy.FDataStore.GetAllPackages()
	orPanic(err)
	proxy := goproxy.NewProxyHttpServer()
	proxy.Logger = &NopLogger{}
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
		HandleConnect(goproxy.AlwaysMitm)
	proxy.OnRequest(goproxy.DstHostIs("package.elm-lang.org:443")).DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if log.GetLevel() > log.InfoLevel {
			log.Debug("Receiving request for ", r.URL.Path)
			// Write requests to file in trace mode.
			if log.GetLevel() == log.TraceLevel {
				reqId := strings.Replace(r.URL.Path, "/", "_", -1)
				b, _ := httputil.DumpRequestOut(r, true)
				*r = *r.WithContext(context.WithValue(r.Context(), "id", reqId))
				ioutil.WriteFile(fmt.Sprintf("./output/request_%s.txt", reqId), b, 0777)
			}
		}

		if resp := mux(r); resp != nil {
			return r, resp
		}
		return r, nil
	})
	// enable curl -p for all hosts on port 80
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*:80$"))).
		HijackConnect(func(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
			defer func() {
				if e := recover(); e != nil {
					ctx.Logf("error connecting to remote: %v", e)
					client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
				}
				client.Close()
			}()
			clientBuf := bufio.NewReadWriter(bufio.NewReader(client), bufio.NewWriter(client))
			remote, err := net.Dial("tcp", req.URL.Host)
			orPanic(err)
			client.Write([]byte("HTTP/1.1 200 Ok\r\n\r\n"))
			remoteBuf := bufio.NewReadWriter(bufio.NewReader(remote), bufio.NewWriter(remote))
			log.Debug("=============" + req.URL.Path + "=====================")
			for {
				req, err := http.ReadRequest(clientBuf.Reader)
				orPanic(err)
				orPanic(req.Write(remoteBuf))
				orPanic(remoteBuf.Flush())
				resp, err := http.ReadResponse(remoteBuf.Reader, req)
				orPanic(err)
				orPanic(resp.Write(clientBuf.Writer))
				orPanic(clientBuf.Flush())
			}
		})
	proxy.OnResponse(goproxy.DstHostIs("package.elm-lang.org:443")).DoFunc(func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if log.GetLevel() == log.TraceLevel {
			old, err := ioutil.ReadAll(r.Body)
			r.Body = ioutil.NopCloser(bytes.NewBuffer(old))
			b, err := httputil.DumpResponse(r, true)
			if err != nil {
				log.Fatal(err.Error())
			}
			r.Body = ioutil.NopCloser(bytes.NewBuffer(old))
			reqid := r.Request.Context().Value("id").(string)
			ioutil.WriteFile(fmt.Sprintf("./output/response-%s.txt", reqid), b, 0777)
		}
		return r
	})
	verbose := flag.Bool("v", true, "should every proxy request be logged to stdout")
	addr := flag.String("addr", ":8080", "proxy listen address")
	flag.Parse()
	proxy.Verbose = *verbose
	log.Printf("Starting server on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, proxy))
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
