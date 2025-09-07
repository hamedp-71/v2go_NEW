package main

import (
    "bufio"
    "context"
    "encoding/base64"
    "encoding/json" // <--- این باید اضافه شود
    "fmt"
    "io"
    "net"           // <--- این باید اضافه شود
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"
)

const (
	timeout         = 20 * time.Second
	maxWorkers      = 10
	maxLinesPerFile = 500
)

var fixedText = `#//profile-title: base64:2YfZhduM2LTZhyDZgdi52KfZhCDwn5iO8J+YjvCfmI4gaGFtZWRwNzE=
#//profile-update-interval: 1
#//subscription-userinfo: upload=0; download=76235908096; total=1486058684416; expire=1767212999
#support-url: https://github.com/hamedp-71/v2go_NEW
#profile-web-page-url: https://github.com/hamedp-71/v2go_NEW
`
// ساختار پاسخ از سرویس GeoIP برای دیکد کردن جیسون
type GeoIPResponse struct {
	CountryCode string `json:"countryCode"`
	Status      string `json:"status"`
}

var protocols = []string{"vmess", "vless", "trojan", "ss", "ssr", "hy2", "tuic", "warp://"}

var links = []string{
	"https://raw.githubusercontent.com/ALIILAPRO/v2rayNG-Config/main/sub.txt",
	"https://raw.githubusercontent.com/mfuu/v2ray/master/v2ray",
	"https://raw.githubusercontent.com/ts-sf/fly/main/v2",
	"https://raw.githubusercontent.com/aiboboxx/v2rayfree/main/v2",
	"https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mci/sub_1.txt",
	"https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mci/sub_2.txt",
	"https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mci/sub_3.txt",
	"https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/app/sub.txt",
	"https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mtn/sub_1.txt",
	"https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mtn/sub_2.txt",
	"https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mtn/sub_3.txt",
	"https://raw.githubusercontent.com/mahsanet/MahsaFreeConfig/refs/heads/main/mtn/sub_4.txt",
	"https://raw.githubusercontent.com/yebekhe/vpn-fail/refs/heads/main/sub-link",
	"https://shadowmere.xyz/api/b64sub/",
	"https://raw.githubusercontent.com/Surfboardv2ray/TGParse/main/splitted/mixed",
}

var dirLinks = []string{
	"https://raw.githubusercontent.com/itsyebekhe/PSG/main/lite/subscriptions/xray/normal/mix",
	"https://raw.githubusercontent.com/HosseinKoofi/GO_V2rayCollector/main/mixed_iran.txt",
	"https://raw.githubusercontent.com/arshiacomplus/v2rayExtractor/refs/heads/main/mix/sub.html",
	"https://raw.githubusercontent.com/darkvpnapp/CloudflarePlus/refs/heads/main/proxy",
	"https://raw.githubusercontent.com/Rayan-Config/C-Sub/refs/heads/main/configs/proxy.txt",
	"https://raw.githubusercontent.com/roosterkid/openproxylist/main/V2RAY_RAW.txt",
	"https://raw.githubusercontent.com/NiREvil/vless/main/sub/SSTime",
	"https://raw.githubusercontent.com/hamedp-71/Trojan/refs/heads/main/hp.txt",
	"https://raw.githubusercontent.com/mahdibland/ShadowsocksAggregator/master/Eternity.txt",
	"https://raw.githubusercontent.com/peweza/SUB-PUBLIC/refs/heads/main/PewezaVPN",
	"https://raw.githubusercontent.com/Everyday-VPN/Everyday-VPN/main/subscription/main.txt",
	"https://raw.githubusercontent.com/MahsaNetConfigTopic/config/refs/heads/main/xray_final.txt",
	"https://github.com/Epodonios/v2ray-configs/raw/main/All_Configs_Sub.txt",
}

type Result struct {
	Content  string
	IsBase64 bool
}

// ===================================================================================
// START: کدهای جدید برای تغییر نام و افزودن پرچم
// ===================================================================================

// countryCodeToFlag converts a two-letter country code to a flag emoji.
// countryCodeToFlag یک کد دو حرفی کشور را به اموجی پرچم تبدیل می‌کند.
func countryCodeToFlag(code string) string {
	if len(code) != 2 {
		return "❓"
	}
	code = strings.ToUpper(code)
	
	// تبدیل هر حرف به یک rune (کاراکتر یونیکد)
	var r1 rune = 0x1F1E6 + rune(code[0]) - 'A'
	var r2 rune = 0x1F1E6 + rune(code[1]) - 'A'

	return string(r1) + string(r2)
}

// getCountryFlag fetches the country flag for a given server address (IP or domain).
func getCountryFlag(address string, client *http.Client) string {
	// First, check if it's a domain or IP
	ip := net.ParseIP(address)
	if ip == nil {
		// It's a domain, resolve it
		ips, err := net.LookupIP(address)
		if err != nil || len(ips) == 0 {
			return "❓" // Cannot resolve domain
		}
		ip = ips[0]
	}

	// Use ip-api.com to get country code
	apiURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,countryCode", ip.String())
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "❓"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return "❓"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "❓"
	}

	var geoInfo GeoIPResponse
	if err := json.Unmarshal(body, &geoInfo); err != nil || geoInfo.Status != "success" {
		return "❓"
	}

	return countryCodeToFlag(geoInfo.CountryCode)
}

// renameConfig decodes a V2Ray config, changes its name ("ps"), and re-encodes it.
func renameConfig(configLink string, client *http.Client) (string, error) {
	parts := strings.SplitN(configLink, "://", 2)
	if len(parts) != 2 {
		return configLink, fmt.Errorf("invalid config link format")
	}
	protocol := parts[0]
	encodedData := parts[1]

	// Handle potential hash (#) in the link
	if strings.Contains(encodedData, "#") {
		encodedData = strings.SplitN(encodedData, "#", 2)[0]
	}

	decodedBytes, err := base64.RawURLEncoding.DecodeString(encodedData)
	if err != nil {
		// Try standard decoding as a fallback
		decodedBytes, err = base64.StdEncoding.DecodeString(encodedData)
		if err != nil {
			return configLink, fmt.Errorf("base64 decoding failed")
		}
	}

	var configData map[string]interface{}
	if err := json.Unmarshal(decodedBytes, &configData); err != nil {
		// If it's not JSON (like Trojan links), just return the original
		return configLink, nil
	}

	// Extract server address ("add")
	address, ok := configData["add"].(string)
	if !ok || address == "" {
		return configLink, fmt.Errorf("address field not found")
	}

	// Get the flag
	flag := getCountryFlag(address, client)

	// Set the new name
	configData["ps"] = fmt.Sprintf("hamedp71-%s", flag)

	modifiedJSON, err := json.Marshal(configData)
	if err != nil {
		return configLink, fmt.Errorf("JSON marshaling failed")
	}

	newEncodedData := base64.StdEncoding.EncodeToString(modifiedJSON)
	return fmt.Sprintf("%s://%s", protocol, newEncodedData), nil
}

// ===================================================================================
// END: کدهای جدید
// ===================================================================================


func main() {
	fmt.Println("Starting V2Ray config aggregator...")

	// Ensure directories exist
	base64Folder, err := ensureDirectoriesExist()
	if err != nil {
		fmt.Printf("Error creating directories: %v\n", err)
		return
	}

	// Create HTTP client with connection pooling
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	// Fetch all URLs concurrently
	fmt.Println("Fetching configurations from sources...")
	allConfigs := fetchAllConfigs(client, links, dirLinks)

	// Filter for protocols
	fmt.Println("Filtering configurations and removing duplicates...")
	originalCount := len(allConfigs)
	filteredConfigs := filterForProtocols(allConfigs, protocols)

	fmt.Printf("Found %d unique valid configurations\n", len(filteredConfigs))
	fmt.Printf("Removed %d duplicates\n", originalCount-len(filteredConfigs))

    // ===================================================================================
	// START: بخش جدید برای تغییر نام کانفیگ‌ها
	// ===================================================================================
	fmt.Println("Renaming configurations and adding country flags...")
	var wg sync.WaitGroup
	renamedChan := make(chan string, len(filteredConfigs))
	semaphore := make(chan struct{}, maxWorkers)

	for _, config := range filteredConfigs {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			newName, err := renameConfig(c, client)
			if err != nil {
				// If renaming fails, use the original config
				renamedChan <- c
			} else {
				renamedChan <- newName
			}
		}(config)
	}

	wg.Wait()
	close(renamedChan)

	var renamedConfigs []string
	for renamed := range renamedChan {
		renamedConfigs = append(renamedConfigs, renamed)
	}
	fmt.Printf("Finished renaming %d configurations.\n", len(renamedConfigs))
	// ===================================================================================
	// END: بخش تغییر نام
	// ===================================================================================
	
	// Clean existing files
	cleanExistingFiles(base64Folder)

	// حالا از کانفیگ‌های تغییرنام‌یافته استفاده می‌کنیم
	mainOutputFile := "All_Configs_Sub.txt"
	err = writeMainConfigFile(mainOutputFile, renamedConfigs)
	if err != nil {
		fmt.Printf("Error writing main config file: %v\n", err)
		return
	}

	fmt.Println("Splitting into smaller files...")
	// اینجا هم از کانفیگ‌های تغییرنام‌یافته استفاده می‌کنیم
	err = splitIntoFiles(base64Folder, renamedConfigs)
	if err != nil {
		fmt.Printf("Error splitting files: %v\n", err)
		return
	}

	fmt.Println("Configuration aggregation completed successfully!")

	sortConfigs()
}

func ensureDirectoriesExist() (string, error) {
	// Create Base64 directory in current directory
	base64Folder := "Base64"
	if err := os.MkdirAll(base64Folder, 0755); err != nil {
		return "", err
	}

	return base64Folder, nil
}

func fetchAllConfigs(client *http.Client, base64Links, textLinks []string) []string {
	var wg sync.WaitGroup
	resultChan := make(chan Result, len(base64Links)+len(textLinks))

	// Worker pool for concurrent requests
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
				resultChan <- Result{Content: content, IsBase64: false}
			}
		}(link)
	}

	// Close channel when all goroutines are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
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

	// Try to decode base64
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
	// Add padding if necessary
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
	seen := make(map[string]bool) // Track duplicates

	for _, line := range data {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip if we've already seen this config
		if seen[line] {
			continue
		}

		for _, protocol := range protocols {
			if strings.HasPrefix(line, protocol) {
				filtered = append(filtered, line)
				seen[line] = true // Mark as seen
				break
			}
		}
	}
	return filtered
}

func cleanExistingFiles(base64Folder string) {
	// Remove main files
	os.Remove("All_Configs_Sub.txt")
	os.Remove("All_Configs_base64_Sub.txt")

	// Remove split files
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

	// Write fixed text
	if _, err := writer.WriteString(fixedText); err != nil {
		return err
	}

	// Write configs
	for _, config := range configs {
		if _, err := writer.WriteString(config + "\n"); err != nil {
			return err
		}
	}

	return nil
}

func splitIntoFiles(base64Folder string, configs []string) error {
	numFiles := (len(configs) + maxLinesPerFile - 1) / maxLinesPerFile

	// Reverse configs so newest go into Sub1, Sub2, etc.
	reversedConfigs := make([]string, len(configs))
	for i, config := range configs {
		reversedConfigs[len(configs)-1-i] = config
	}

	for i := 0; i < numFiles; i++ {
		// Create custom header for this file
		profileTitle := fmt.Sprintf("🆓 Git:DanialSamadi | Sub%d 🔥", i+1)
		encodedTitle := base64.StdEncoding.EncodeToString([]byte(profileTitle))
		customFixedText := fmt.Sprintf(`#//profile-title: base64:Sub%d
#//profile-update-interval: 1
#//subscription-userinfo: upload=0; download=76235908096; total=1486058684416; expire=1767212999
#support-url: https://github.com/hamedp-71/v2go_NEW
#profile-web-page-url: https://github.com/hamedp-71/v2go_NEW
`, encodedTitle)

		// Calculate slice bounds (using reversed configs)
		start := i * maxLinesPerFile
		end := start + maxLinesPerFile
		if end > len(reversedConfigs) {
			end = len(reversedConfigs)
		}

		// Write regular file (in current directory)
		filename := fmt.Sprintf("Sub%d.txt", i+1)
		if err := writeSubFile(filename, customFixedText, reversedConfigs[start:end]); err != nil {
			return err
		}

		// Read the file and create base64 version
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

	// Write header
	if _, err := writer.WriteString(header); err != nil {
		return err
	}

	// Write configs
	for _, config := range configs {
		if _, err := writer.WriteString(config + "\n"); err != nil {
			return err
		}
	}

	return nil
}
