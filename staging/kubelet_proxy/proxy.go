package kubelet_proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultHeaderRequestTimeout = 10 * time.Second
)

// StartKubeletProxy initializes and starts the kubelet proxy server.
func StartKubeletProxy(
	ctx context.Context,
	certFile, keyFile string,
	listenAddr string,
	clientSet kubernetes.Interface,
) error {
	proxy, err := NewPodHandler(clientSet)
	if err != nil {
		return err
	}

	tlsConfig, err := loadTLSConfig(certFile, keyFile)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	proxy.AttachPodRoutes(mux)

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: DefaultHeaderRequestTimeout,
	}

	go func() {
		log.G(ctx).Infof("Starting kubelet proxy server on %s", listenAddr)
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.G(ctx).Fatalf("Failed to start kubelet proxy server: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		log.G(ctx).Info("Shutting down kubelet proxy server")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.G(ctx).Errorf("Failed to gracefully shutdown kubelet proxy server: %v", err)
		}
	}()

	return nil
}

func loadTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("fail to load TLS certificate and key: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
