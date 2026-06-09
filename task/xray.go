package task

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type XrayConfig struct {
	Inbounds  []XrayInbound  `json:"inbounds"`
	Outbounds []XrayOutbound `json:"outbounds"`
}

type XrayInbound struct {
	Port     int            `json:"port"`
	Protocol string         `json:"protocol"`
	Settings map[string]any `json:"settings"`
}

type XrayOutbound struct {
	Protocol       string         `json:"protocol"`
	Settings       map[string]any `json:"settings"`
	StreamSettings map[string]any `json:"streamSettings,omitempty"`
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func ParseToXrayConfig(uri string, localPort int) (*XrayConfig, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	cfg := &XrayConfig{
		Inbounds: []XrayInbound{
			{
				Port:     localPort,
				Protocol: "socks",
				Settings: map[string]any{"udp": true},
			},
		},
		Outbounds: []XrayOutbound{},
	}

	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}
	host := u.Hostname()
	passwordOrUUID := ""
	if u.User != nil {
		passwordOrUUID = u.User.Username()
	}

	q := u.Query()
	network := q.Get("type")
	if network == "" {
		network = "tcp"
	}
	security := q.Get("security")

	streamSettings := map[string]any{
		"network":  network,
		"security": security,
	}

	if security == "tls" || security == "reality" {
		sni := q.Get("sni")
		if sni == "" {
			sni = host
		}
		tlsSettings := map[string]any{
			"serverName":  sni,
			"fingerprint": q.Get("fp"),
		}
		if security == "reality" {
			tlsSettings["publicKey"] = q.Get("pbk")
			tlsSettings["shortId"] = q.Get("sid")
			streamSettings["realitySettings"] = tlsSettings
		} else {
			streamSettings["tlsSettings"] = tlsSettings
		}
	}

	if network == "ws" {
		wsPath := q.Get("path")
		if wsPath == "" {
			wsPath = "/"
		}
		wsHost := q.Get("host")
		if wsHost == "" {
			wsHost = host
		}
		streamSettings["wsSettings"] = map[string]any{
			"path": wsPath,
			"headers": map[string]string{
				"Host": wsHost,
			},
		}
	} else if network == "grpc" {
		streamSettings["grpcSettings"] = map[string]any{
			"serviceName": q.Get("serviceName"),
			"multiMode":   q.Get("mode") == "multi",
		}
	}

	out := XrayOutbound{
		Protocol:       u.Scheme,
		StreamSettings: streamSettings,
	}

	if u.Scheme == "vless" {
		encryption := q.Get("encryption")
		if encryption == "" {
			encryption = "none"
		}
		out.Settings = map[string]any{
			"vnext": []map[string]any{
				{
					"address": host,
					"port":    port,
					"users": []map[string]any{
						{
							"id":         passwordOrUUID,
							"encryption": encryption,
						},
					},
				},
			},
		}
	} else if u.Scheme == "trojan" {
		out.Settings = map[string]any{
			"servers": []map[string]any{
				{
					"address":  host,
					"port":     port,
					"password": passwordOrUUID,
				},
			},
		}
	} else {
		return nil, errors.New("unsupported protocol: " + u.Scheme)
	}

	cfg.Outbounds = append(cfg.Outbounds, out)
	return cfg, nil
}

func TestRealDelay(uri string) (int, error) {
	if _, err := os.Stat("xray.exe"); os.IsNotExist(err) {
		return -1, errors.New("xray.exe not found")
	}

	port, err := getFreePort()
	if err != nil {
		return -1, err
	}

	cfg, err := ParseToXrayConfig(uri, port)
	if err != nil {
		return -1, err
	}

	cfgBytes, _ := json.Marshal(cfg)
	
	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("xray_%d_%x.json", time.Now().UnixNano(), randBytes))
	os.WriteFile(tmpFile, cfgBytes, 0644)
	defer os.Remove(tmpFile)

	cwd, _ := os.Getwd()
	xrayName := "xray"
	if runtime.GOOS == "windows" {
		xrayName = "xray.exe"
	}
	xrayPath := filepath.Join(cwd, xrayName)
	cmd := exec.Command(xrayPath, "run", "-config", tmpFile)
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Wait a bit for xray to start
	time.Sleep(500 * time.Millisecond)

	proxyUrl, _ := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", port))
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 3 * time.Second,
		}).DialContext,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	// First request to warm up the connection (TCP handshake, TLS etc)
	client.Get("https://www.google.com/generate_204")

	start := time.Now()
	resp, err := client.Get("https://www.google.com/generate_204")
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		return int(time.Since(start).Milliseconds()), nil
	}

	return -1, errors.New("bad status code")
}
