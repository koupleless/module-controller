package kubelet_proxy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestStartKubeletProxy(t *testing.T) {
	cert, key := genTestCertPair()
	defer func() {
		os.Remove(cert)
		os.Remove(key)
	}()

	t.Run("normal case", func(t *testing.T) {
		ctx, cancelFunc := context.WithCancel(context.Background())
		defer cancelFunc()

		err := StartKubeletProxy(ctx, cert, key, ":10250", fake.NewClientset())
		require.NoError(t, err)
	})

	t.Run("negative cases", func(t *testing.T) {
		t.Run("empty cert file", func(t *testing.T) {
			ctx, cancelFunc := context.WithCancel(context.Background())
			defer cancelFunc()

			err := StartKubeletProxy(ctx, "", key, ":10250", fake.NewClientset())
			require.Error(t, err)
		})
		t.Run("empty key file", func(t *testing.T) {
			ctx, cancelFunc := context.WithCancel(context.Background())
			defer cancelFunc()

			err := StartKubeletProxy(ctx, cert, "", ":10250", fake.NewClientset())
			require.Error(t, err)
		})
	})
}

func genTestCertPair() (certFileLoc string, keyFileLoc string) {
	certFileLoc = "test_cert.pem"
	keyFileLoc = "test_key.pem"

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("failed to generate private key: " + err.Error())
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(8760 * time.Hour), // 365 天
		DNSNames:     []string{"localhost"},
	}
	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		panic("failed to create certificate: " + err.Error())
	}
	certFile, err := os.Create(certFileLoc)
	if err != nil {
		panic("failed to create cert file: " + err.Error())
	}
	defer certFile.Close()
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert})
	if err != nil {
		panic("failed to write cert to file: " + err.Error())
	}
	keyFile, err := os.Create(keyFileLoc)
	if err != nil {
		panic("failed to create key file: " + err.Error())
	}
	defer keyFile.Close()
	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	if err != nil {
		panic("failed to write key to file: " + err.Error())
	}
	return certFileLoc, keyFileLoc
}
