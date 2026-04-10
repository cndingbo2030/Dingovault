// Package network provides LAN discovery (mDNS) and PIN-based credential exchange for sync setup.
package network

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

// PairingCredentials is exchanged after a successful PIN check (plaintext on LAN — use only on trusted networks).
type PairingCredentials struct {
	WebDAVURL        string `json:"webdavUrl"`
	WebDAVUser       string `json:"webdavUser"`
	WebDAVPassword   string `json:"webdavPassword"`
	WebDAVRemoteRoot string `json:"webdavRemoteRoot,omitempty"`
	S3Region         string `json:"s3Region,omitempty"`
	S3Bucket         string `json:"s3Bucket,omitempty"`
	S3Prefix         string `json:"s3Prefix,omitempty"`
	S3AccessKey      string `json:"s3AccessKey,omitempty"`
	S3SecretKey      string `json:"s3SecretKey,omitempty"`
	S3Endpoint       string `json:"s3Endpoint,omitempty"`
}

// StartPINAdvertiser listens for pairing connections, publishes mDNS, and returns a 4-digit PIN.
// stop closes the listener and mDNS server; it is safe to call more than once.
func StartPINAdvertiser(ctx context.Context, instance string, pairingPort int, cred PairingCredentials) (pin string, stop func(), err error) {
	if pairingPort <= 0 {
		pairingPort = 17375
	}
	if strings.TrimSpace(instance) == "" {
		h, _ := os.Hostname()
		instance = strings.TrimSpace(h)
		if instance == "" {
			instance = "dingovault"
		}
	}
	pin, err = randomPIN4()
	if err != nil {
		return "", nil, err
	}
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", fmt.Sprintf("0.0.0.0:%d", pairingPort))
	if err != nil {
		return "", nil, fmt.Errorf("pairing listen: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handlePairingConn(conn, pin, cred)
		}
	}()

	txt := []string{"dv=1", "role=sync"}
	zone, err := mdns.NewMDNSService(instance, DingovaultService, "", "", pairingPort, nil, txt)
	if err != nil {
		_ = ln.Close()
		wg.Wait()
		return "", nil, fmt.Errorf("mdns service: %w", err)
	}
	srv, err := mdns.NewServer(&mdns.Config{Zone: zone})
	if err != nil {
		_ = ln.Close()
		wg.Wait()
		return "", nil, fmt.Errorf("mdns server: %w", err)
	}

	stop = func() {
		_ = srv.Shutdown()
		_ = ln.Close()
		wg.Wait()
	}
	return pin, stop, nil
}

func handlePairingConn(conn net.Conn, expectedPin string, cred PairingCredentials) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	rd := bufio.NewReader(conn)
	line, err := rd.ReadString('\n')
	if err != nil {
		_, _ = conn.Write([]byte("ERR read\n"))
		return
	}
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "pin") || fields[1] != expectedPin {
		_, _ = conn.Write([]byte("ERR pin\n"))
		return
	}
	_, _ = conn.Write([]byte("OK\n"))
	enc := json.NewEncoder(conn)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(cred); err != nil {
		return
	}
}

// PairWithPeer connects to a discovered host, sends the PIN, and decodes sync credentials (WebDAV and optional S3).
func PairWithPeer(ctx context.Context, host string, port int, pin string) (PairingCredentials, error) {
	var zero PairingCredentials
	if port <= 0 {
		return zero, fmt.Errorf("invalid port")
	}
	pin = strings.TrimSpace(pin)
	if len(pin) != 4 {
		return zero, fmt.Errorf("pin must be 4 digits")
	}
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		return zero, err
	}
	defer conn.Close()
	_, err = fmt.Fprintf(conn, "PIN %s\n", pin)
	if err != nil {
		return zero, err
	}
	rd := bufio.NewReader(conn)
	status, err := rd.ReadString('\n')
	if err != nil {
		return zero, err
	}
	if !strings.HasPrefix(strings.TrimSpace(status), "OK") {
		return zero, fmt.Errorf("pairing rejected: %s", strings.TrimSpace(status))
	}
	var cred PairingCredentials
	if err := json.NewDecoder(rd).Decode(&cred); err != nil {
		return zero, err
	}
	return cred, nil
}

func randomPIN4() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", 1000+n.Int64()), nil
}
