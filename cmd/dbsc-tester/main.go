// Dbsc-tester is a tiny HTTPS app to exercise Device Bound Session Credentials in Chrome.
//
// TLS: for a trusted localhost certificate (no --ignore-certificate-errors), use mkcert
// and pass the PEM paths:
//
//	mkcert -install
//	mkcert localhost 127.0.0.1 ::1
//	go run ./cmd/dbsc-tester -cert=./localhost+2.pem -key=./localhost+2-key.pem
//
// Without -cert/-key, a short-lived ephemeral cert is used (Chrome needs --ignore-certificate-errors).
//
// Run:
//
//	go run ./cmd/dbsc-tester
//
// Then open Chrome with DBSC enabled, for example:
//
//	/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
//	  --enable-features=DeviceBoundSessions \
//	  --ignore-certificate-errors \
//	  https://localhost:8443/
//
// Flow: visit /login (sets session + triggers DBSC registration offer on save), then /protected; /logout clears the session.
// Use -log-level=debug (default) to see session/dbsc registration traces on stderr.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"lds.li/web/requestlog"
	"lds.li/web/session"
)

func main() {
	addr := flag.String("addr", "localhost:8443", "listen address (HTTPS)")
	certFile := flag.String("cert", "", "TLS certificate PEM (e.g. from mkcert); must be used with -key")
	keyFile := flag.String("key", "", "TLS private key PEM; must be used with -cert")
	refresh := flag.Duration("dbsc-refresh", 5*time.Minute, "DBSC proof refresh interval")
	logLevel := flag.String("log-level", "debug", "slog level: debug, info, warn, error")
	flag.Parse()

	lvl := parseLogLevel(*logLevel)
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	appLog := slog.New(h)
	slog.SetDefault(appLog)

	if (*certFile == "") != (*keyFile == "") {
		log.Fatal("tls: both -cert and -key are required together")
	}

	var cert tls.Certificate
	var err error
	if *certFile != "" {
		cert, err = tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("tls: load %q / %q: %v", *certFile, *keyFile, err)
		}
	} else {
		cert, err = ephemeralLocalhostCert()
		if err != nil {
			log.Fatalf("tls: ephemeral cert: %v", err)
		}
	}

	kv := session.NewMemoryKV()
	mgr, err := session.NewKVManager(kv, &session.ManagerOpts{
		IdleTimeout:            24 * time.Hour,
		DBSCRefreshInterval:    *refresh,
		DBSCRegistrationPath:   "/register",
		DBSCRefreshPath:        "/dbsc/refresh",
	})
	if err != nil {
		log.Fatalf("session manager: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/logout", handleLogout)
	mux.HandleFunc("/protected", handleProtected)
	mux.HandleFunc("/dbsc/refresh", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logging := &requestlog.RequestLogger{Logger: appLog}
	srv := &http.Server{
		Addr:    *addr,
		Handler: logging.Handler(mgr.Wrap(mux)),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	tlsLn := tls.NewListener(ln, srv.TLSConfig)

	appLog.Info("dbsc-tester starting", "addr", *addr, "log_level", lvl.String())
	if *certFile != "" {
		appLog.Info("tls", "cert", *certFile, "key", *keyFile)
	} else {
		appLog.Info("tls", "mode", "ephemeral", "hint", "use mkcert + -cert/-key to skip --ignore-certificate-errors in Chrome")
	}
	appLog.Info("chrome hint", "flags", "--enable-features=DeviceBoundSessions")
	log.Fatal(srv.Serve(tlsLn))
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		log.Fatalf("unknown -log-level %q (want debug, info, warn, error)", s)
		return slog.LevelInfo // unreachable
	}
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = homeTmpl.Execute(w, nil)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	sess, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "no session", http.StatusInternalServerError)
		return
	}
	sess.Set("user", "dbsc-tester")
	sess.Set("login_at", time.Now().Format(time.RFC3339))
	http.Redirect(w, r, "/protected", http.StatusSeeOther)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "no session", http.StatusInternalServerError)
		return
	}
	sess.Delete()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleProtected(w http.ResponseWriter, r *http.Request) {
	sess, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "no session", http.StatusInternalServerError)
		return
	}
	if sess.Get("user") == nil {
		http.Error(w, "visit /login first", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = protectedTmpl.Execute(w, struct {
		User    any
		Bound   bool
		LoginAt any
	}{
		User:    sess.Get("user"),
		Bound:   sess.IsDeviceBound(),
		LoginAt: sess.Get("login_at"),
	})
}

var homeTmpl = template.Must(template.New("home").Parse(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>dbsc-tester</title></head>
<body>
<h1>dbsc-tester</h1>
<p><a href="/login">/login</a> — start session (triggers DBSC registration on save)</p>
<p><a href="/logout">/logout</a> — clear session cookie</p>
<p><a href="/protected">/protected</a> — needs login; may challenge DBSC after refresh interval</p>
<p>Registration POSTs to <code>/register</code> are handled by session middleware (no app handler).</p>
</body>
</html>`))

var protectedTmpl = template.Must(template.New("protected").Parse(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>protected</title></head>
<body>
<h1>Protected</h1>
<p>User: {{.User}}</p>
<p>Logged in at: {{.LoginAt}}</p>
<p>Device-bound (DBSC): <strong>{{.Bound}}</strong></p>
<p><a href="/">home</a> · <a href="/logout">logout</a></p>
</body>
</html>`))

func ephemeralLocalhostCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"dbsc-tester dev"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create cert: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return tls.X509KeyPair(certPEM, keyPEM)
}
