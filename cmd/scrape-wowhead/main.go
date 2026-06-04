// cmd/scrape-wowhead/main.go
//
// One-time scraper to build data/item_specs.json from Wowhead.
// Run from the project root:
//
//	go run ./cmd/scrape-wowhead/ [flags]
//
// Flags:
//
//	--data    path to directory containing items.lua / dungeons.lua  (default: ./data)
//	--out     output JSON file path                                   (default: ./data/item_specs.json)
//	--delay   milliseconds to wait between requests                  (default: 1200)
//	--proxy   SOCKS5 proxy address                                   (default: "" = direct)
//	--rotate  rotate Tor circuit every N items (0 = never)          (default: 0)
//
// Tor / Brave examples:
//
//	# Tor Browser or system Tor (default SOCKS5 port)
//	go run ./cmd/scrape-wowhead/ --proxy 127.0.0.1:9050
//
//	# Brave's built-in Tor window (uses a different port)
//	go run ./cmd/scrape-wowhead/ --proxy 127.0.0.1:9150
//
//	# Rotate the Tor exit circuit every 20 items (requires Tor ControlPort on 9051)
//	go run ./cmd/scrape-wowhead/ --proxy 127.0.0.1:9050 --rotate 20
//
// The file is written incrementally: if it already exists the scraper skips
// items that are already present, so you can safely resume after a failure or
// partial run.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// ── flags ────────────────────────────────────────────────────────────────────

var (
	flagData   = flag.String("data", "./data", "directory with items.lua / dungeons.lua")
	flagOut    = flag.String("out", "./data/item_specs.json", "output JSON path")
	flagDelay  = flag.Int("delay", 1200, "milliseconds between requests")
	flagProxy  = flag.String("proxy", "", "SOCKS5 proxy address, e.g. 127.0.0.1:9050 for Tor")
	flagRotate = flag.Int("rotate", 0, "rotate Tor circuit every N items via ControlPort (0 = off)")
)

// ── types ────────────────────────────────────────────────────────────────────

// ItemSpecMap is the on-disk format: map[itemID] → []specID
type ItemSpecMap map[int][]int

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()

	// 1. Build the HTTP client (optionally through Tor/SOCKS5).
	client, err := buildClient(*flagProxy)
	if err != nil {
		fatalf("building HTTP client: %v", err)
	}

	if *flagProxy != "" {
		fmt.Printf("Using SOCKS5 proxy: %s\n", *flagProxy)
		// Quick connectivity check via Tor's IP-echo service.
		if ip, err := checkTorIP(client); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] proxy check failed: %v\n", err)
		} else {
			fmt.Printf("Proxy exit IP: %s\n", ip)
		}
	}

	// 2. Collect all item IDs referenced across all dungeons in Lua files.
	itemIDs, err := collectItemIDs(*flagData)
	if err != nil {
		fatalf("collecting item IDs: %v", err)
	}
	fmt.Printf("Found %d unique item IDs in loot tables\n", len(itemIDs))

	// 3. Load any existing output so we can resume.
	existing := loadExisting(*flagOut)
	fmt.Printf("Already scraped: %d items — skipping those\n", len(existing))

	// 4. Determine which items still need scraping.
	var todo []int
	for _, id := range itemIDs {
		if _, done := existing[id]; !done {
			todo = append(todo, id)
		}
	}
	fmt.Printf("Items to scrape: %d\n", len(todo))

	if len(todo) == 0 {
		fmt.Println("Nothing to do. item_specs.json is up to date.")
		return
	}

	delay := time.Duration(*flagDelay) * time.Millisecond
	done := 0
	errors := 0

	for _, id := range todo {
		// Rotate Tor circuit before fetching if requested.
		if *flagRotate > 0 && done > 0 && done%*flagRotate == 0 {
			fmt.Println("  [TOR] Rotating circuit...")
			if err := rotateTorCircuit(); err != nil {
				fmt.Fprintf(os.Stderr, "  [WARN] circuit rotation failed: %v\n", err)
			} else {
				time.Sleep(3 * time.Second) // give Tor a moment to establish the new circuit
			}
		}

		specs, err := scrapeSpecs(client, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [WARN] item %d: %v\n", id, err)
			errors++
			existing[id] = []int{}
		} else {
			existing[id] = specs
		}

		done++
		fmt.Printf("  [%d/%d] item %d → %v\n", done, len(todo), id, existing[id])

		// Write after every item so progress is never lost.
		if err := saveJSON(*flagOut, existing); err != nil {
			fatalf("writing output: %v", err)
		}

		if done < len(todo) {
			time.Sleep(delay)
		}
	}

	fmt.Printf("\nDone. %d scraped, %d errors. Output: %s\n", done, errors, *flagOut)
}

// ── HTTP client ───────────────────────────────────────────────────────────────

// buildClient returns an *http.Client that routes through a SOCKS5 proxy when
// proxyAddr is non-empty, or a plain client otherwise.
func buildClient(proxyAddr string) (*http.Client, error) {
	if proxyAddr == "" {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 dialer: %w", err)
	}

	var transport *http.Transport

	// golang.org/x/net/proxy dialers implement proxy.ContextDialer since
	// x/net v0.0.0-20210119194325-5f4716e94777 — prefer it when available
	// so context cancellation works properly.
	if cd, ok := dialer.(proxy.ContextDialer); ok {
		transport = &http.Transport{
			DialContext: cd.DialContext,
		}
	} else {
		// Fallback: ignore the context and use the plain Dial method.
		transport = &http.Transport{
			DialContext: func(_ context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second, // Tor is slower — allow more time
	}, nil
}

// checkTorIP hits a plain-text IP echo endpoint to confirm the proxy is
// working and show which exit node we're using.
func checkTorIP(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// ── Tor circuit rotation ──────────────────────────────────────────────────────

// rotateTorCircuit sends SIGNAL NEWNYM to the Tor ControlPort (127.0.0.1:9051).
// This requests a new exit circuit — the IP seen by Wowhead will change.
//
// Requires ControlPort to be enabled in torrc:
//
//	ControlPort 9051
//	CookieAuthentication 0   (or set a password and update this function)
func rotateTorCircuit() error {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:9051", 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect to ControlPort: %w", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Authenticate (no password / cookie auth).
	if _, err := fmt.Fprintf(conn, "AUTHENTICATE\r\n"); err != nil {
		return err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "250") {
		return fmt.Errorf("authenticate: unexpected response: %s", strings.TrimSpace(line))
	}

	// Send NEWNYM signal.
	if _, err := fmt.Fprintf(conn, "SIGNAL NEWNYM\r\n"); err != nil {
		return err
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "250") {
		return fmt.Errorf("NEWNYM: unexpected response: %s", strings.TrimSpace(line))
	}

	return nil
}

// ── scraping ─────────────────────────────────────────────────────────────────

var specsRe = regexp.MustCompile(`"specs":\[([0-9,]+)\]`)

func scrapeSpecs(client *http.Client, itemID int) ([]int, error) {
	url := fmt.Sprintf("https://www.wowhead.com/item=%d", itemID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:109.0) Gecko/20100101 Firefox/115.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited (429) — increase --delay or rotate circuit")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseSpecs(string(body)), nil
}

func parseSpecs(html string) []int {
	m := specsRe.FindStringSubmatch(html)
	if m == nil {
		return []int{}
	}

	var specs []int
	for _, s := range strings.Split(m[1], ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.Atoi(s)
		if err == nil && id > 0 {
			specs = append(specs, id)
		}
	}
	return specs
}

// ── loot ID collection ────────────────────────────────────────────────────────

func collectItemIDs(dataDir string) ([]int, error) {
	seen := make(map[int]bool)

	itemIDRe := regexp.MustCompile(`^\s*\[(\d+)\]\s*=\s*\{`)
	raw, err := os.ReadFile(dataDir + "/items.lua")
	if err != nil {
		return nil, fmt.Errorf("items.lua: %w", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		m := itemIDRe.FindStringSubmatch(line)
		if m != nil {
			id, _ := strconv.Atoi(m[1])
			seen[id] = true
		}
	}

	lootRe := regexp.MustCompile(`lootTable\s*=\s*\{([^}]+)\}`)
	numRe := regexp.MustCompile(`\d+`)
	stripComment := regexp.MustCompile(`--\[\[.*?\]\]`)

	raw2, err := os.ReadFile(dataDir + "/dungeons.lua")
	if err != nil {
		return nil, fmt.Errorf("dungeons.lua: %w", err)
	}
	for _, line := range strings.Split(string(raw2), "\n") {
		line = stripComment.ReplaceAllString(line, "")
		m := lootRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, s := range numRe.FindAllString(m[1], -1) {
			id, _ := strconv.Atoi(s)
			if id > 0 {
				seen[id] = true
			}
		}
	}

	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids, nil
}

// ── JSON helpers ──────────────────────────────────────────────────────────────

func loadExisting(path string) ItemSpecMap {
	out := make(ItemSpecMap)
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	raw := make(map[string][]int)
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] could not parse existing %s: %v — starting fresh\n", path, err)
		return out
	}
	for k, v := range raw {
		id, err := strconv.Atoi(k)
		if err == nil {
			out[id] = v
		}
	}
	return out
}

func saveJSON(path string, m ItemSpecMap) error {
	raw := make(map[string][]int, len(m))
	for k, v := range m {
		raw[strconv.Itoa(k)] = v
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}
