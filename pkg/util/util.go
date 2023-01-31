package util

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"github.com/spaolacci/murmur3"
	"github.com/yhy0/Jie/logging"
	"net"
	"net/url"
	"reflect"
	"sort"
	"strings"
)

func UUID() (UUID string) {
	uuid := uuid.New()
	UUID = uuid.String()
	return
}

func Contains(stringA, stringB string) bool {
	// stringA 原始串，stringB 要查找的字串
	return strings.Contains(strings.ToLower(stringA), strings.ToLower(stringB))
}

// RemoveDuplicateElement  数组去重
func RemoveDuplicateElement(strs []string) []string {
	var result []string
	for _, item := range strs {
		item = strings.TrimSpace(item)
		if item != "" && !In(item, result) {
			item = strings.TrimSpace(item)
			result = append(result, item)
		}
	}
	return result
}

func Difference(slice1, slice2 []string) []string {
	intersect := func(slice1, slice2 []string) []string {
		m := make(map[string]int)
		nn := make([]string, 0)
		for _, v := range slice1 {
			m[v]++
		}

		for _, v := range slice2 {
			times, _ := m[v]
			if times == 1 {
				nn = append(nn, v)
			}
		}
		return nn
	}

	m := make(map[string]int)
	nn := make([]string, 0)
	inter := intersect(slice1, slice2)
	for _, v := range inter {
		m[v]++
	}

	for _, value := range slice1 {
		times, _ := m[value]
		if times == 0 {
			nn = append(nn, value)
		}
	}
	return nn
}

// In 判断一个字符串是否在另一个字符数组里面，存在返回true
// target 中是否包含 strs 中的某一个
// 这个是 target 范围大， element只是一个子串
func In(target string, strs []string) bool {
	for _, element := range strs {
		element = strings.TrimSpace(element)
		if Contains(target, element) {
			return true
		}
	}
	return false
}

// In1 这个是 element 范围大， target 只是一个子串
func In1(target string, strs []string) bool {
	for _, element := range strs {
		if Contains(element, target) {
			return true
		}
	}
	return false
}

func IntInSlice(i int, slice []int) bool {
	if slice == nil {
		return false
	}
	sort.Ints(slice)
	index := sort.SearchInts(slice, i)
	if index < len(slice) && slice[index] == i {
		return true
	}
	return false
}

// ArrayToString 数组转字符串，空格分开
func ArrayToString(arr []string) string {
	var result string
	for l, i := range arr { //遍历数组中所有元素追加成string
		if l == 0 || len(arr) == 1 {
			result = i
		} else {
			result = result + " " + i
		}
	}
	return result
}

// IsInnerIP 判断是否为内网IP
func IsInnerIP(ip string) bool {
	if ip == "localhost" {
		return true
	}
	IP := net.ParseIP(ip)
	if ip4 := IP.To4(); ip4 != nil {
		if ip4.IsLoopback() || ip4.IsLinkLocalMulticast() || ip4.IsLinkLocalUnicast() {
			return true
		}
		return ip4.IsPrivate()
	}
	return false
}

// Cidr2IPs C段转ip
func Cidr2IPs(cidr string) []string {
	var ips []string

	ipAddr, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		logging.Logger.Print(err)
	}

	for ip := ipAddr.Mask(ipNet.Mask); ipNet.Contains(ip); increment(ip) {
		ips = append(ips, ip.String())
	}
	return ips
}

func increment(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func StructureIps(ipsTmp []string, num int) (ips []string) {
	ipMap := make(map[string]int)

	for _, ip := range ipsTmp {
		front := ip[:strings.LastIndex(ip, ".")] + ".0"

		if _, ok := ipMap[front]; ok {
			ipMap[front] += 1
		} else {
			ipMap[front] = 1
		}
	}

	for k, v := range ipMap {
		if v > num { // 一个 ip 段大于 15 个ip，才会将此段加入检测列表中
			ips = append(ips, Cidr2IPs(k+"/24")...)
		}
	}

	ipMap = nil

	ips = append(ips, ipsTmp...)

	return RemoveDuplicateElement(ips)
}

func ToStringSlice(actual interface{}) ([]string, error) {
	var res []string
	value := reflect.ValueOf(actual)
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return nil, errors.New("parse error")
	}
	for i := 0; i < value.Len(); i++ {
		res = append(res, value.Index(i).Interface().(string))
	}
	return res, nil

}

func CheckErrs(err error) bool {
	if err == nil {
		return false
	}
	errs := []string{
		"closed by the remote host", "too many connections",
		"i/o timeout", "EOF", "A connection attempt failed",
		"established connection failed", "connection attempt failed",
		"Unable to read", "is not allowed to connect to this",
		"no pg_hba.conf entry",
		"No connection could be made",
		"invalid packet size",
		"bad connection",
	}
	for _, key := range errs {
		if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(key)) {
			return true
		}
	}
	return false
}

func Hostname(target string) string {
	u, err := url.Parse(target)
	if err != nil {
		logging.Logger.Errorln("Hostname: ", err)
		return target
	}

	return u.Hostname()
}

func FormatTarget(target string) (targets []string) {
	targets_tmp := strings.Split(target, "\n")

	if strings.Contains(target, ",") {
		for _, t := range targets_tmp {
			ts := strings.Split(t, ",")
			targets = append(targets, ts...)
		}
	} else {
		targets = targets_tmp
	}

	return RemoveDuplicateElement(targets)
}

// IsIPv6 returns true when the provided net.IP address is an IPv6 address.
func IsIPv6(ip string) bool {
	return strings.Count(ip, ":") >= 2
}

type UserPass struct {
	Username string
	Password string
}

func CvtUps(s string) []UserPass {
	a := strings.Split(s, "\n")
	var aRst []UserPass
	for _, x := range a {
		x = strings.TrimSpace(x)
		if "" == x {
			continue
		}
		j := strings.Split(x, ",")
		if 1 < len(j) {
			aRst = append(aRst, UserPass{Username: j[0], Password: j[1]})
		}
	}
	return aRst
}
func CvtLines(s string) []string {
	return strings.Split(s, "\n")
}

func FaviconHash(data []byte) int32 {
	stdBase64 := base64.StdEncoding.EncodeToString(data)
	stdBase64 = InsertInto(stdBase64, 76, '\n')
	hasher := murmur3.New32WithSeed(0)
	hasher.Write([]byte(stdBase64))
	return int32(hasher.Sum32())
}

func InsertInto(s string, interval int, sep rune) string {
	var buffer bytes.Buffer
	before := interval - 1
	last := len(s) - 1
	for i, char := range s {
		buffer.WriteRune(char)
		if i%interval == before && i != last {
			buffer.WriteRune(sep)
		}
	}
	buffer.WriteRune(sep)
	return buffer.String()
}

func ToUpper(a []string) []string {
	var b []string
	for _, v := range a {
		b = append(b, strings.ToUpper(v))
	}
	return b
}

// MD5 获取字符串md5
func MD5(str string) string {
	c := md5.New()
	c.Write([]byte(str))
	bytes := c.Sum(nil)
	return hex.EncodeToString(bytes)
}

// ReverseString 反向string
func ReverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
