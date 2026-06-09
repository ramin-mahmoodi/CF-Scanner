package task

import (
	"crypto/tls"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/utils"
)

const (
	tcpConnectTimeout = time.Second * 2
	maxRoutine        = 1000
	defaultRoutines   = 200
	defaultPort       = 443
	defaultPingTimes  = 4
)

var (
	Routines      = defaultRoutines
	TCPPort   int = defaultPort
	PingTimes int = defaultPingTimes
	SNI       string = "speed.cloudflare.com"
)

type Ping struct {
	wg       *sync.WaitGroup
	m        *sync.Mutex
	ipChan   <-chan *net.IPAddr
	csv      utils.PingDelaySet
	control  chan bool
	bar      *utils.Bar
	totalIPs int
}

func checkPingDefault() {
	if Routines <= 0 {
		Routines = defaultRoutines
	}
	if TCPPort <= 0 || TCPPort >= 65535 {
		TCPPort = defaultPort
	}
	if PingTimes <= 0 {
		PingTimes = defaultPingTimes
	}
}

func NewPing() *Ping {
	checkPingDefault()
	ipChan, total := GenerateIPs()
	return &Ping{
		wg:       &sync.WaitGroup{},
		m:        &sync.Mutex{},
		ipChan:   ipChan,
		csv:      make(utils.PingDelaySet, 0),
		control:  make(chan bool, Routines),
		bar:      utils.NewBar(total, "Available: ", ""),
		totalIPs: total,
	}
}

func (p *Ping) Run() utils.PingDelaySet {
	if p.totalIPs == 0 {
		return p.csv
	}

	utils.Cyan.Printf("Start smart scan (Port: %d, Range: %v ~ %v ms, Loss: %.2f)\n", TCPPort, utils.InputMinDelay.Milliseconds(), utils.InputMaxDelay.Milliseconds(), utils.InputMaxLossRate)

	for ip := range p.ipChan {
		if utils.CancelCtx != nil && utils.CancelCtx.Err() != nil {
			break
		}
		p.wg.Add(1)
		p.control <- false
		go p.start(ip)
	}
	p.wg.Wait()
	p.bar.Done()
	sort.Sort(p.csv)
	return p.csv
}

func (p *Ping) start(ip *net.IPAddr) {
	defer p.wg.Done()
	if utils.CancelCtx != nil && utils.CancelCtx.Err() != nil {
		<-p.control
		return
	}
	p.tcpingHandler(ip)
	<-p.control
}

// handle tcping
func (p *Ping) tcpingHandler(ip *net.IPAddr) {
	recv, tcpDelayTotal, jitter, tlsDelay, httpDelay, colo := p.smartPing(ip)
	nowAble := len(p.csv)
	if recv != 0 {
		nowAble++
	}
	p.bar.Grow(1, strconv.Itoa(nowAble))
	if recv == 0 {
		return
	}
	
	avgTCP := tcpDelayTotal / time.Duration(recv)
	
	// Composite Score calculation:
	lossRate := float64(PingTimes-recv) / float64(PingTimes)
	delayMs := float64(avgTCP.Milliseconds())
	jitterMs := float64(jitter.Microseconds()) / 1000.0

	// Add TLS and HTTP delays to the score penalty
	stages := 1.0
	if tlsDelay > 0 {
		delayMs += float64(tlsDelay.Milliseconds())
		stages += 1.0
	}

	if httpDelay > 0 {
		delayMs += float64(httpDelay.Milliseconds())
		stages += 1.0
	}
	normalizedDelayMs := delayMs / stages

	score := 100.0 - (lossRate * 100.0) - (normalizedDelayMs / 10.0) - (jitterMs / 10.0)
	if score < 0 {
		score = 0
	}

	data := &utils.PingData{
		IP:       ip,
		Sended:   PingTimes,
		Received: recv,
		Delay:    avgTCP,
		Jitter:   jitter,
		Colo:     colo,
	}

	p.m.Lock()
	defer p.m.Unlock()
	ipData := utils.CloudflareIPData{
		PingData: data,
		Score:    score,
	}
	p.csv = append(p.csv, ipData)
	if utils.GuiLiveCallback != nil {
		utils.GuiLiveCallback(ipData)
	}
}

func (p *Ping) smartPing(ip *net.IPAddr) (recv int, tcpDelayTotal time.Duration, jitter time.Duration, tlsDelay time.Duration, httpDelay time.Duration, colo string) {
	var lastDelay time.Duration
	var totalJitter time.Duration
	var jitterCount int

	var fullAddress string
	if isIPv4(ip.String()) {
		fullAddress = fmt.Sprintf("%s:%d", ip.String(), TCPPort)
	} else {
		fullAddress = fmt.Sprintf("[%s]:%d", ip.String(), TCPPort)
	}

	for i := 0; i < PingTimes; i++ {
		if utils.CancelCtx != nil && utils.CancelCtx.Err() != nil {
			break
		}
		startTime := time.Now()
		conn, err := net.DialTimeout("tcp", fullAddress, tcpConnectTimeout)
		if err == nil {
			delay := time.Since(startTime)
			recv++
			tcpDelayTotal += delay
			if lastDelay > 0 {
				diff := delay - lastDelay
				if diff < 0 {
					diff = -diff
				}
				totalJitter += diff
				jitterCount++
			}
			lastDelay = delay
			conn.Close()
		}
	}

	if recv == 0 {
		return
	}
	if jitterCount > 0 {
		jitter = totalJitter / time.Duration(jitterCount)
	}

	tlsHttpConn, err := net.DialTimeout("tcp", fullAddress, tcpConnectTimeout)
	if err == nil {
		defer tlsHttpConn.Close()

		scheme := "https"
		if TCPPort == 80 || TCPPort == 8080 || TCPPort == 8880 || TCPPort == 2052 || TCPPort == 2082 || TCPPort == 2086 || TCPPort == 2095 {
			scheme = "http"
		}

		var activeConn net.Conn = tlsHttpConn

		if scheme == "https" && TestTLS {
			startTime := time.Now()
			tlsConn := tls.Client(tlsHttpConn, &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         SNI,
			})
			tlsHttpConn.SetDeadline(time.Now().Add(2 * time.Second))
			if err := tlsConn.Handshake(); err == nil {
				tlsDelay = time.Since(startTime)
				activeConn = tlsConn
			} else {
				return // Failed TLS
			}
		}

		if TestHTTP {
			startTime := time.Now()
			reqStr := fmt.Sprintf("GET /__down?bytes=0 HTTP/1.1\r\nHost: %s\r\nUser-Agent: Mozilla/5.0\r\nConnection: close\r\n\r\n", SNI)
			req := []byte(reqStr)
			activeConn.SetDeadline(time.Now().Add(2 * time.Second))
			if _, err := activeConn.Write(req); err == nil {
				buf := make([]byte, 1024)
				n, _ := activeConn.Read(buf)
				if n > 0 {
					httpDelay = time.Since(startTime)
					resp := string(buf[:n])
					// Extract Colo if possible (Cf-Ray: xxx-COLO)
					if idx := strings.Index(resp, "CF-RAY: "); idx != -1 {
						if end := strings.Index(resp[idx:], "\r\n"); end != -1 {
							ray := resp[idx+8 : idx+end]
							if dash := strings.LastIndex(ray, "-"); dash != -1 {
								colo = strings.ToUpper(strings.TrimSpace(ray[dash+1:]))
							}
						}
					}
				}
			}
		}
	}
	return
}
