package task

import (
	"bufio"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultInputFile = "ip.txt"

var (
	IPsPerRange int    = 20
	ScanMode    string = "random"

	// TestAll test all ip
	TestAll = false
	// IPFile is the filename of IP Rangs
	IPFile = defaultInputFile
	IPText string
)

func InitRandSeed() {
	rand.Seed(time.Now().UnixNano())
}

func isIPv4(ip string) bool {
	return strings.Contains(ip, ".")
}

func randIPEndWith(num byte) byte {
	if num == 0 { // 对于 /32 这种单独的 IP
		return byte(0)
	}
	return byte(rand.Intn(int(num)))
}

type IPRanges struct {
	ips     []*net.IPAddr
	mask    string
	firstIP net.IP
	ipNet   *net.IPNet
}

func newIPRanges() *IPRanges {
	return &IPRanges{
		ips: make([]*net.IPAddr, 0),
	}
}

// 如果是单独 IP 则加上子网掩码，反之则获取子网掩码(r.mask)
func (r *IPRanges) fixIP(ip string) string {
	// 如果不含有 '/' 则代表不是 IP 段，而是一个单独的 IP，因此需要加上 /32 /128 子网掩码
	if i := strings.IndexByte(ip, '/'); i < 0 {
		if isIPv4(ip) {
			r.mask = "/32"
		} else {
			r.mask = "/128"
		}
		ip += r.mask
	} else {
		r.mask = ip[i:]
	}
	return ip
}

// 解析 IP 段，获得 IP、IP 范围、子网掩码
func (r *IPRanges) parseCIDR(ip string) {
	var err error
	if r.firstIP, r.ipNet, err = net.ParseCIDR(r.fixIP(ip)); err != nil {
		log.Fatalln("ParseCIDR err", err)
	}
}

func (r *IPRanges) appendIPv4(d byte) {
	r.appendIP(net.IPv4(r.firstIP[12], r.firstIP[13], r.firstIP[14], d))
}

func (r *IPRanges) appendIP(ip net.IP) {
	r.ips = append(r.ips, &net.IPAddr{IP: ip})
}

// 返回第四段 ip 的最小值及可用数目
func (r *IPRanges) getIPRange() (minIP, hosts byte) {
	minIP = r.firstIP[15] & r.ipNet.Mask[3] // IP 第四段最小值

	// 根据子网掩码获取主机数量
	m := net.IPv4Mask(255, 255, 255, 255)
	for i, v := range r.ipNet.Mask {
		m[i] ^= v
	}
	total, _ := strconv.ParseInt(m.String(), 16, 32) // 总可用 IP 数
	if total > 255 {                                 // 矫正 第四段 可用 IP 数
		hosts = 255
		return
	}
	hosts = byte(total)
	return
}

func (r *IPRanges) chooseIPv4() {
	if r.mask == "/32" {
		r.appendIP(r.firstIP)
		return
	}

	if ScanMode == "sequential" {
		count := 0
		ip := make(net.IP, len(r.firstIP))
		copy(ip, r.firstIP)
		ip[15]++ // skip network address
		for r.ipNet.Contains(ip) && count < IPsPerRange {
			targetIP := make([]byte, len(ip))
			copy(targetIP, ip)
			r.appendIP(targetIP)
			count++

			for i := 15; i >= 12; i-- {
				ip[i]++
				if ip[i] != 0 {
					break
				}
			}
		}
	} else {
		ones, bits := r.ipNet.Mask.Size()
		hostBits := bits - ones
		var totalHosts uint32 = 1 << hostBits

		count := 0
		picked := make(map[uint32]bool)
		maxAttempts := IPsPerRange * 50
		attempts := 0

		baseIP := uint32(r.firstIP[12])<<24 | uint32(r.firstIP[13])<<16 | uint32(r.firstIP[14])<<8 | uint32(r.firstIP[15])

		for count < IPsPerRange && attempts < maxAttempts {
			attempts++
			var offset uint32
			if totalHosts <= 2 {
				offset = 0
			} else {
				offset = 1 + uint32(rand.Intn(int(totalHosts-2)))
			}

			if !picked[offset] {
				picked[offset] = true
				targetInt := baseIP + offset
				targetIP := net.IPv4(byte(targetInt>>24), byte(targetInt>>16), byte(targetInt>>8), byte(targetInt))
				if r.ipNet.Contains(targetIP) {
					r.appendIP(targetIP)
					count++
				}
				if totalHosts <= 2 {
					break
				}
			}
		}
	}
}

func (r *IPRanges) chooseIPv6() {
	if r.mask == "/128" {
		r.appendIP(r.firstIP)
		return
	}

	if ScanMode == "sequential" {
		count := 0
		ip := make(net.IP, len(r.firstIP))
		copy(ip, r.firstIP)
		ip[15]++ // skip network address
		for r.ipNet.Contains(ip) && count < IPsPerRange {
			targetIP := make([]byte, len(ip))
			copy(targetIP, ip)
			r.appendIP(targetIP)
			count++

			for i := 15; i >= 0; i-- {
				ip[i]++
				if ip[i] != 0 {
					break
				}
			}
		}
	} else {
		count := 0
		maxAttempts := IPsPerRange * 50
		attempts := 0
		picked := make(map[string]bool)

		for count < IPsPerRange && attempts < maxAttempts {
			attempts++
			targetIP := make(net.IP, len(r.firstIP))
			copy(targetIP, r.firstIP)

			ones, _ := r.ipNet.Mask.Size()
			for i := 15; i >= ones/8; i-- {
				if i == ones/8 {
					maskByte := r.ipNet.Mask[i]
					targetIP[i] = (targetIP[i] & maskByte) | (byte(rand.Intn(256)) & ^maskByte)
				} else {
					targetIP[i] = byte(rand.Intn(256))
				}
			}

			if !picked[string(targetIP)] && r.ipNet.Contains(targetIP) {
				picked[string(targetIP)] = true
				r.appendIP(targetIP)
				count++
			}
		}
	}
}

func GenerateIPs() (<-chan *net.IPAddr, int) {
	ch := make(chan *net.IPAddr, 100)
	ranges := newIPRanges()
	
	// Collect strings to parse
	var cidrs []string
	if IPText != "" {
		IPs := strings.Split(IPText, ",")
		for _, IP := range IPs {
			IP = strings.TrimSpace(IP)
			if IP != "" {
				cidrs = append(cidrs, IP)
			}
		}
	} else {
		if IPFile == "" {
			IPFile = defaultInputFile
		}
		if file, err := os.Open(IPFile); err == nil {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					cidrs = append(cidrs, line)
				}
			}
			file.Close()
		} else {
			log.Fatal(err)
		}
	}

	total := len(cidrs) * IPsPerRange

	go func() {
		defer close(ch)
		for _, cidr := range cidrs {
			ranges.parseCIDR(cidr)
			if isIPv4(cidr) {
				ranges.chooseIPv4()
			} else {
				ranges.chooseIPv6()
			}
			for _, ip := range ranges.ips {
				ch <- ip
			}
			ranges.ips = make([]*net.IPAddr, 0) // Reset for the next CIDR to free memory
		}
	}()

	return ch, total
}
