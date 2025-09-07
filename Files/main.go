package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ... (تمام بخش‌های ثابت مثل timeout, maxWorkers, GeoIPResponse, fixedText, protocols, links, dirLinks بدون تغییر باقی می‌مانند)
const (
	timeout         = 20 * time.Second
	maxWorkers      = 20
	maxLinesPerFile = 500
)

type GeoIPResponse struct {
	CountryCode string `json:"countryCode"`
	Status      string `json:"status"`
}
// یک کش برای ذخیره پرچم کشورها بر اساس IP تا درخواست‌های تکراری ارسال نشود
var ipToFlagCache = sync.Map{}

var fixedText = `#//profile-title: base64:2YfZhduM2LTZhyDZgdi52KfZhCDwn5iO8J+YjvCfmI4gaGFtZWRwNzE=
#//profile-update-interval: 1
#//subscription-userinfo: upload=0; download=76235908096; total=1486058684416; expire=1767212999
#support-url: https://github.com/hamedp-71/v2go_NEW
#profile-web-page-url: https://github.com/hamedp-71/v2go_NEW
`

var protocols = []string{"vmess", "vless", "trojan", "ss", "ssr", "hy2", "tuic", "warp://"}

var links = []string{"https://raw.githubusercontent.com/ALIILAPRO/v2rayNG-Config/main/sub.txt", "https://raw.githubusercontent.com/mfuu/v2ray/master/v2ray", "https://raw.githubusercontent.com/ts-sf/fly/main/v2", "https://raw.githubusercontent.com/aiboboxx/v2rayfree/main/v2", "https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mci/sub_1.txt", "https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mci/sub_2.txt", "https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mci/sub_3.txt", "https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/app/sub.txt", "https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mtn/sub_1.txt", "https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mtn/sub_2.txt", "https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mtn/sub_3.txt", "https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mtn/sub_4.txt", "https://raw.githubusercontent.com/yebekhe/vpn-fail/refs/heads/main/sub-link", "https://shadowmere.xyz/api/b64sub/", "https://raw.githubusercontent.com/Surfboardv2ray/TGParse/main/splitted/mixed"}

var dirLinks = []string{"https://raw.githubusercontent.com/itsyebekhe/PSG/main/lite/subscriptions/xray/normal/mix", "https://raw.githubusercontent.com/HosseinKoofi/GO_V2rayCollector/main/mixed_iran.txt", "https://raw.githubusercontent.com/arshiacomplus/v2rayExtractor/refs/heads/main/mix/sub.html", "https://raw.githubusercontent.com/darkvpnapp/CloudflarePlus/refs/heads/main/proxy", "https://raw.githubusercontent.com/Rayan-Config/C-Sub/refs/heads/main/configs/proxy.txt", "https://raw.githubusercontent.com/roosterkid/openproxylist/main/V2RAY_RAW.txt", "https://raw.githubusercontent.com/NiREvil/vless/main/sub/SSTime", "https://raw.githubusercontent.com/hamedp-71/Trojan/refs/heads/main/hp.txt", "https://raw.githubusercontent.com/mahdibland/ShadowsocksAggregator/master/Eternity.txt", "https://raw.githubusercontent.com/peweza/SUB-PUBLIC/refs/heads/main/PewezaVPN", "https://raw.githubusercontent.com/Everyday-VPN/Everyday-VPN/main/subscription/main.txt", "https://raw.githubusercontent.com/MahsaNetConfigTopic/config/refs/heads/main/xray_final.txt", "https://github.com/Epodonios/v2ray-configs/raw/main/All_Configs_Sub.txt"}

type Result struct {
	Content  string
	IsBase64 bool
}

// =================== START: کد اصلاح شده برای عیب‌یابی ===================

func countryCodeToFlag(code string) string {
	if len(code) != 2 {
		return "❓"
	}
	code = strings.ToUpper(code)
	var r1 rune = 0x1F1E6 + rune(code[0]) - 'A'
	var r2 rune = 0x1F1E6 + rune(code[1]) - 'A'
	return string(r1) + string(r2)
}

func getCountryFlag(address string, client *http.Client) (string, error) {
	ip := net.ParseIP(address)
	if ip == nil {
		ips, err := net.LookupIP(address)
		if err != nil || len(ips) == 0 {
			return "", fmt.Errorf("DNS lookup failed for %s", address) // بازگرداندن خطا
		}
		ip = ips[0]
	}

	apiURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,countryCode", ip.String())
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call to ip-api.com failed: %v", err) // بازگرداندن خطا
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var geoInfo GeoIPResponse
	if err := json.Unmarshal(body, &geoInfo); err != nil || geoInfo.Status != "success" {
		return "", fmt.Errorf("failed to parse GeoIP response or status not success for %s", address) // بازگرداندن خطا
	}

	return countryCodeToFlag(geoInfo.CountryCode), nil
}

// renameConfig کانفیگ‌های VLESS/VMess, Trojan و SS را تغییر نام می‌دهد
func renameConfig(configLink string, client *http.Client) (string, error) {
	parts := strings.SplitN(configLink, "://", 2)
	if len(parts) != 2 {
		return configLink, fmt.Errorf("invalid format")
	}
	protocol := parts[0]
	mainPart := strings.SplitN(parts[1], "#", 2)[0]
	
	var address string

	switch protocol {
	case "vless", "vmess":
		decodedBytes, err := base64.RawURLEncoding.DecodeString(mainPart)
		if err != nil {
			decodedBytes, err = base64.StdEncoding.DecodeString(mainPart)
			if err != nil {
				return configLink, fmt.Errorf("base64 decoding failed")
			}
		}

		var configData map[string]interface{}
		if err := json.Unmarshal(decodedBytes, &configData); err != nil {
			return configLink, fmt.Errorf("not a JSON-based config")
		}

		addr, ok := configData["add"].(string)
		if !ok || addr == "" {
			return configLink, fmt.Errorf("address field ('add') not found")
		}
		address = addr

	case "trojan", "ss":
		// ساختار: protocol://credentials@address:port
		atParts := strings.SplitN(mainPart, "@", 2)
		if len(atParts) != 2 {
			return configLink, fmt.Errorf("invalid %s format", protocol)
		}
		addrPort := atParts[1]
		address = strings.SplitN(addrPort, ":", 2)[0]

	default:
		// پروتکل‌های دیگر مثل ssr, tuic پشتیبانی نمی‌شوند
		return configLink, fmt.Errorf("unsupported protocol for renaming: %s", protocol)
	}

	// حالا که آدرس را داریم، پرچم را می‌گیریم (با استفاده از کش)
	if flag, ok := ipToFlagCache.Load(address); ok {
		// اگر در کش بود، از همان استفاده کن
		return buildNewLink(protocol, mainPart, flag.(string)), nil
	}

	// اگر در کش نبود، از شبکه بگیر
	flag, err := getCountryFlag(address, client)
	if err != nil {
		return configLink, fmt.Errorf("could not get flag for %s: %v", address, err)
	}
	ipToFlagCache.Store(address, flag) // نتیجه را در کش ذخیره کن
	return buildNewLink(protocol, mainPart, flag), nil
}

// تابع کمکی برای ساخت لینک نهایی
func buildNewLink(protocol, mainPart, flag string) string {
	newName := fmt.Sprintf("hamedp71-%s", flag)
	// برای VLESS/VMess، باید JSON را ویرایش کنیم که پیچیده است.
	// برای سادگی، فعلاً فقط نام را با # اضافه می‌کنیم که در اکثر کلاینت‌ها کار می‌کند.
	return fmt.Sprintf("%s://%s#%s", protocol, mainPart, newName)
}
// buildNewLink یک تابع کمکی برای ساخت لینک نهایی با نام جدید است.
// این تابع برای جلوگیری از تکرار کد استفاده می‌شود.
func buildNewLink(protocol, mainPart, flag string) string {
    newName := fmt.Sprintf("hamedp71-%s", flag)

    // برای پروتکل‌های مبتنی بر JSON، باید JSON را ویرایش کنیم.
    if protocol == "vless" || protocol == "vmess" {
        decodedBytes, err := base64.RawURLEncoding.DecodeString(mainPart)
        if err != nil {
            decodedBytes, _ = base64.StdEncoding.DecodeString(mainPart)
        }

        if decodedBytes != nil {
            var configData map[string]interface{}
            if json.Unmarshal(decodedBytes, &configData) == nil {
                configData["ps"] = newName
                if modifiedJSON, err := json.Marshal(configData); err == nil {
                    newEncodedData := base64.StdEncoding.EncodeToString(modifiedJSON)
                    return fmt.Sprintf("%s://%s", protocol, newEncodedData)
                }
            }
        }
    }

    // برای پروتکل‌های دیگر (Trojan, SS) یا در صورت خطا، نام را با # اضافه می‌کنیم.
    return fmt.Sprintf("%s://%s#%s", protocol, mainPart, newName)
}

func main() {
	// ... (بخش اولیه main مثل قبل) ...
	fmt.Println("Starting V2Ray config aggregator...")
	base64Folder, err := ensureDirectoriesExist()
	if err != nil {
		fmt.Printf("Error creating directories: %v\n", err)
		return
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{MaxIdleConns: 100, MaxIdleConnsPerHost: 10, IdleConnTimeout: 30 * time.Second, DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext},
	}
	fmt.Println("Fetching configurations from sources...")
	allConfigs := fetchAllConfigs(client, links, dirLinks)
	fmt.Println("Filtering configurations and removing duplicates...")
	originalCount := len(allConfigs)
	filteredConfigs := filterForProtocols(allConfigs, protocols)
	fmt.Printf("Found %d unique valid configurations\n", len(filteredConfigs))
	fmt.Printf("Removed %d duplicates\n", originalCount-len(filteredConfigs))

	// =================== START: بخش تغییر یافته main برای لاگ‌گیری ===================
	fmt.Println("Renaming configurations and adding country flags...")
	var wg sync.WaitGroup
	renamedChan := make(chan string, len(filteredConfigs))
	semaphore := make(chan struct{}, maxWorkers)
	var successCount, failCount int32
	var mu sync.Mutex // برای جلوگیری از چاپ همزمان و درهم

	for _, config := range filteredConfigs {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			newName, err := renameConfig(c, client)
			if err != nil {
				// اگر تغییر نام شکست خورد، دلیلش را چاپ کن
				mu.Lock()
				//fmt.Printf("[FAIL] Config: %.30s... Reason: %v\n", c, err)
				failCount++
				mu.Unlock()
				renamedChan <- c // از کانفیگ اصلی استفاده کن
			} else {
				mu.Lock()
				successCount++
				mu.Unlock()
				renamedChan <- newName
			}
		}(config)
	}

	wg.Wait()
	close(renamedChan)

	// چاپ گزارش نهایی
	fmt.Printf("\n--- Renaming Summary ---\n")
	fmt.Printf("Successful renames: %d\n", successCount)
	fmt.Printf("Failed renames:     %d\n", failCount)
	fmt.Printf("------------------------\n\n")

	var renamedConfigs []string
	for renamed := range renamedChan {
		renamedConfigs = append(renamedConfigs, renamed)
	}
	// =================== END: بخش تغییر یافته main برای لاگ‌گیری ===================

	cleanExistingFiles(base64Folder)
	mainOutputFile := "All_Configs_Sub.txt"
	err = writeMainConfigFile(mainOutputFile, renamedConfigs)
	if err != nil {
		fmt.Printf("Error writing main config file: %v\n", err)
		return
	}
	fmt.Println("Splitting into smaller files...")
	err = splitIntoFiles(base64Folder, renamedConfigs)
	if err != nil {
		fmt.Printf("Error splitting files: %v\n", err)
		return
	}
	fmt.Println("Configuration aggregation completed successfully!")
	// sortConfigs() // اگر این تابع در فایل دیگری است، آن را فعال کنید
}

// ... (تمام توابع دیگر مثل ensureDirectoriesExist, fetchAllConfigs و ... بدون تغییر باقی می‌مانند)
// ... (Please include all other functions from your previous code here)
func ensureDirectoriesExist() (string, error) {
	base64Folder := "Base64"
	if err := os.MkdirAll(base64Folder, 0755); err != nil {
		return "", err
	}
	return base64Folder, nil
}
func fetchAllConfigs(client *http.Client, base64Links, textLinks []string) []string {
	var wg sync.WaitGroup
	resultChan := make(chan Result, len(base64Links)+len(textLinks))
	semaphore := make(chan struct{}, maxWorkers)

	// Fetch base64-encoded links
	for _, link := range base64Links {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			content := fetchAndDecodeBase64(client, url)
			if content != "" {
				resultChan <- Result{Content: content, IsBase64: true}
			}
		}(link)
	}

	// Fetch text links
	for _, link := range textLinks {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			content := fetchText(client, url)
			if content != "" {
				// این خط اصلاح شد: فاصله اضافی حذف شد
				resultChan <- Result{Content: content, IsBase64: false}
			}
		}(link)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allConfigs []string
	for result := range resultChan {
		lines := strings.Split(strings.TrimSpace(result.Content), "\n")
		allConfigs = append(allConfigs, lines...)
	}
	return allConfigs
}
func fetchAndDecodeBase64(client *http.Client, url string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	decoded, err := decodeBase64(body)
	if err != nil {
		return ""
	}
	return decoded
}
func fetchText(client *http.Client, url string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(body)
}
func decodeBase64(encoded []byte) (string, error) {
	encodedStr := string(encoded)
	if len(encodedStr)%4 != 0 {
		encodedStr += strings.Repeat("=", 4-len(encodedStr)%4)
	}
	decoded, err := base64.StdEncoding.DecodeString(encodedStr)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
func filterForProtocols(data []string, protocols []string) []string {
	var filtered []string
	seen := make(map[string]bool)
	for _, line := range data {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if seen[line] {
			continue
		}
		for _, protocol := range protocols {
			if strings.HasPrefix(line, protocol) {
				filtered = append(filtered, line)
				seen[line] = true
				break
			}
		}
	}
	return filtered
}
func cleanExistingFiles(base64Folder string) {
	os.Remove("All_Configs_Sub.txt")
	os.Remove("All_Configs_base64_Sub.txt")
	for i := 0; i < 20; i++ {
		os.Remove(fmt.Sprintf("Sub%d.txt", i))
		os.Remove(filepath.Join(base64Folder, fmt.Sprintf("Sub%d_base64.txt", i)))
	}
}
func writeMainConfigFile(filename string, configs []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	if _, err := writer.WriteString(fixedText); err != nil {
		return err
	}
	for _, config := range configs {
		if _, err := writer.WriteString(config + "\n"); err != nil {
			return err
		}
	}
	return nil
}
func splitIntoFiles(base64Folder string, configs []string) error {
	numFiles := (len(configs) + maxLinesPerFile - 1) / maxLinesPerFile
	reversedConfigs := make([]string, len(configs))
	for i, config := range configs {
		reversedConfigs[len(configs)-1-i] = config
	}
	for i := 0; i < numFiles; i++ {
		profileTitle := fmt.Sprintf("🆓 Git:DanialSamadi | Sub%d 🔥", i+1)
		encodedTitle := base64.StdEncoding.EncodeToString([]byte(profileTitle))
		customFixedText := fmt.Sprintf(`#//profile-title: base64:%s
#//profile-update-interval: 1
#//subscription-userinfo: upload=0; download=76235908096; total=1486058684416; expire=1767212999
#support-url: https://github.com/hamedp-71/v2go_NEW
#profile-web-page-url: https://github.com/hamedp-71/v2go_NEW
`, encodedTitle)
		start := i * maxLinesPerFile
		end := start + maxLinesPerFile
		if end > len(reversedConfigs) {
			end = len(reversedConfigs)
		}
		filename := fmt.Sprintf("Sub%d.txt", i+1)
		if err := writeSubFile(filename, customFixedText, reversedConfigs[start:end]); err != nil {
			return err
		}
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		base64Filename := filepath.Join(base64Folder, fmt.Sprintf("Sub%d_base64.txt", i+1))
		encodedContent := base64.StdEncoding.EncodeToString(content)
		if err := os.WriteFile(base64Filename, []byte(encodedContent), 0644); err != nil {
			return err
		}
	}
	return nil
}
func writeSubFile(filename, header string, configs []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	if _, err := writer.WriteString(header); err != nil {
		return err
	}
	for _, config := range configs {
		if _, err := writer.WriteString(config + "\n"); err != nil {
			return err
		}
	}
	return nil
}
