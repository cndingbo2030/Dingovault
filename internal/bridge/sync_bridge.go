package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cndingbo2030/dingovault/internal/config"
	"github.com/cndingbo2030/dingovault/internal/locale"
	"github.com/cndingbo2030/dingovault/internal/network"
	vaultsync "github.com/cndingbo2030/dingovault/internal/sync"
)

// SyncSettingsDTO is persisted sync configuration for the Svelte UI.
type SyncSettingsDTO struct {
	WebDAVURL        string `json:"webdavUrl"`
	WebDAVUser       string `json:"webdavUser"`
	WebDAVPassword   string `json:"webdavPassword"`
	WebDAVRemoteRoot string `json:"webdavRemoteRoot"`
	PairingPort      int    `json:"pairingPort"`
	LANInstanceName  string `json:"lanInstanceName"`
	S3Region         string `json:"s3Region"`
	S3Bucket         string `json:"s3Bucket"`
	S3Prefix         string `json:"s3Prefix"`
	S3AccessKey      string `json:"s3AccessKey"`
	S3SecretKey      string `json:"s3SecretKey"`
	S3Endpoint       string `json:"s3Endpoint"`
}

// LANPeerDTO is one discovered Dingovault instance on the LAN.
type LANPeerDTO struct {
	Name string   `json:"name"`
	Host string   `json:"host"`
	IP   string   `json:"ip"`
	Port int      `json:"port"`
	TXT  []string `json:"txt,omitempty"`
}

func syncDTOFromConfig(s config.SyncSettings) SyncSettingsDTO {
	return SyncSettingsDTO{
		WebDAVURL:        s.WebDAVURL,
		WebDAVUser:       s.WebDAVUser,
		WebDAVPassword:   s.WebDAVPassword,
		WebDAVRemoteRoot: s.WebDAVRemoteRoot,
		PairingPort:      s.PairingPort,
		LANInstanceName:  s.LANInstanceName,
		S3Region:         s.S3Region,
		S3Bucket:         s.S3Bucket,
		S3Prefix:         s.S3Prefix,
		S3AccessKey:      s.S3AccessKey,
		S3SecretKey:      s.S3SecretKey,
		S3Endpoint:       s.S3Endpoint,
	}
}

func syncDTOToConfig(d SyncSettingsDTO) config.SyncSettings {
	return config.SyncSettings{
		WebDAVURL:        strings.TrimSpace(d.WebDAVURL),
		WebDAVUser:       strings.TrimSpace(d.WebDAVUser),
		WebDAVPassword:   d.WebDAVPassword,
		WebDAVRemoteRoot: strings.TrimSpace(d.WebDAVRemoteRoot),
		PairingPort:      d.PairingPort,
		LANInstanceName:  strings.TrimSpace(d.LANInstanceName),
		S3Region:         strings.TrimSpace(d.S3Region),
		S3Bucket:         strings.TrimSpace(d.S3Bucket),
		S3Prefix:         strings.TrimSpace(d.S3Prefix),
		S3AccessKey:      strings.TrimSpace(d.S3AccessKey),
		S3SecretKey:      d.S3SecretKey,
		S3Endpoint:       strings.TrimSpace(d.S3Endpoint),
	}
}

// GetSyncSettings returns WebDAV and LAN pairing options (secrets are local-only).
func (a *App) GetSyncSettings() (SyncSettingsDTO, error) {
	c, err := config.Load()
	if err != nil {
		return SyncSettingsDTO{}, err
	}
	return syncDTOFromConfig(config.NormalizeSyncSettings(c.Sync)), nil
}

// SetSyncSettings saves sync options. Empty webdavPassword preserves the previous password.
func (a *App) SetSyncSettings(dto SyncSettingsDTO) error {
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	prev := c.Sync
	next := syncDTOToConfig(dto)
	if strings.TrimSpace(dto.WebDAVPassword) == "" {
		next.WebDAVPassword = prev.WebDAVPassword
	}
	if strings.TrimSpace(dto.S3SecretKey) == "" {
		next.S3SecretKey = prev.S3SecretKey
	}
	c.Sync = config.NormalizeSyncSettings(next)
	return config.Save(c)
}

// SyncVaultWebDAV performs a bidirectional .md sync with the configured WebDAV server.
func (a *App) SyncVaultWebDAV() error {
	if a.notesRoot == "" {
		return fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	c, err := config.Load()
	if err != nil {
		return err
	}
	s := config.NormalizeSyncSettings(c.Sync)
	if strings.TrimSpace(s.WebDAVURL) == "" {
		return fmt.Errorf("webdav url not configured")
	}
	cfg := vaultsync.WebDAVConfig{
		URL:        s.WebDAVURL,
		User:       s.WebDAVUser,
		Password:   s.WebDAVPassword,
		RemoteRoot: s.WebDAVRemoteRoot,
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	if err := vaultsync.SyncMarkdownVault(ctx, a.notesRoot, cfg); err != nil {
		return err
	}
	// Re-read files from disk into the index after remote pulls.
	return a.reindexVaultMarkdown()
}

// SyncVaultS3 performs a bidirectional .md sync with the configured S3 bucket (or S3-compatible API).
func (a *App) SyncVaultS3() error {
	if a.notesRoot == "" {
		return fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	c, err := config.Load()
	if err != nil {
		return err
	}
	s := config.NormalizeSyncSettings(c.Sync)
	if strings.TrimSpace(s.S3Bucket) == "" {
		return fmt.Errorf("s3 bucket not configured")
	}
	cfg := vaultsync.S3Config{
		Region:    s.S3Region,
		Bucket:    s.S3Bucket,
		Prefix:    s.S3Prefix,
		AccessKey: s.S3AccessKey,
		SecretKey: s.S3SecretKey,
		Endpoint:  s.S3Endpoint,
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	if err := vaultsync.SyncMarkdownVaultS3(ctx, a.notesRoot, cfg); err != nil {
		return err
	}
	return a.reindexVaultMarkdown()
}

func (a *App) reindexVaultMarkdown() error {
	if a.graph == nil {
		return nil
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return filepath.WalkDir(a.notesRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		return a.graph.ReindexFile(ctx, path)
	})
}

// ListLANSyncPeers discovers other Dingovault desktops advertising on the LAN.
func (a *App) ListLANSyncPeers() ([]LANPeerDTO, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := network.BrowseDingovaultPeers(ctx, 2*time.Second)
	if err != nil {
		return nil, err
	}
	out := make([]LANPeerDTO, len(raw))
	for i := range raw {
		out[i] = LANPeerDTO{
			Name: raw[i].Name,
			Host: raw[i].Host,
			IP:   raw[i].IP,
			Port: raw[i].Port,
			TXT:  raw[i].TXT,
		}
	}
	return out, nil
}

// StartLANSyncAdvertise opens the PIN pairing server and publishes mDNS; returns the 4-digit PIN to show locally.
func (a *App) StartLANSyncAdvertise() (string, error) {
	a.StopLANSyncAdvertise()
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	s := config.NormalizeSyncSettings(c.Sync)
	name := s.LANInstanceName
	if name == "" {
		h, _ := os.Hostname()
		name = strings.TrimSpace(h)
		if name == "" {
			name = "dingovault"
		}
	}
	cred := network.PairingCredentials{
		WebDAVURL:        s.WebDAVURL,
		WebDAVUser:       s.WebDAVUser,
		WebDAVPassword:   s.WebDAVPassword,
		WebDAVRemoteRoot: s.WebDAVRemoteRoot,
		S3Region:         s.S3Region,
		S3Bucket:         s.S3Bucket,
		S3Prefix:         s.S3Prefix,
		S3AccessKey:      s.S3AccessKey,
		S3SecretKey:      s.S3SecretKey,
		S3Endpoint:       s.S3Endpoint,
	}
	pin, stop, err := network.StartPINAdvertiser(context.Background(), name, s.PairingPort, cred)
	if err != nil {
		return "", err
	}
	a.lanMu.Lock()
	a.stopLAN = stop
	a.lanMu.Unlock()
	return pin, nil
}

// StopLANSyncAdvertise tears down mDNS and the pairing listener.
func (a *App) StopLANSyncAdvertise() {
	a.lanMu.Lock()
	stop := a.stopLAN
	a.stopLAN = nil
	a.lanMu.Unlock()
	if stop != nil {
		stop()
	}
}

// PairLANSyncWith connects to a peer, verifies the PIN, and stores received sync settings (WebDAV and optional S3) locally.
func (a *App) PairLANSyncWith(host string, port int, pin string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("empty host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cred, err := network.PairWithPeer(ctx, host, port, strings.TrimSpace(pin))
	if err != nil {
		return err
	}
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	prev := c.Sync
	c.Sync = config.NormalizeSyncSettings(config.SyncSettings{
		WebDAVURL:        cred.WebDAVURL,
		WebDAVUser:       cred.WebDAVUser,
		WebDAVPassword:   cred.WebDAVPassword,
		WebDAVRemoteRoot: cred.WebDAVRemoteRoot,
		S3Region:         cred.S3Region,
		S3Bucket:         cred.S3Bucket,
		S3Prefix:         cred.S3Prefix,
		S3AccessKey:      cred.S3AccessKey,
		S3SecretKey:      cred.S3SecretKey,
		S3Endpoint:       cred.S3Endpoint,
		PairingPort:      prev.PairingPort,
		LANInstanceName:  prev.LANInstanceName,
	})
	return config.Save(c)
}
